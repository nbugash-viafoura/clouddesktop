---

description: "Task list for Multiple Cloud Desktops per Developer"
---

# Tasks: Multiple Cloud Desktops per Developer

**Input**: Design documents from `specs/002-multi-instance-management/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Included. The developer's standards require TDD for critical business logic, and plan.md research R11 defines the test approach. Write each test task first and confirm it fails (a compile failure counts) before starting the matching implementation task.

**Tracking**: There is no Jira key for this feature, so this spec folder is the decision record.

**Organization**: Tasks are grouped by user story. All four P1 stories are ordered by technical dependency: US4 first, because it moves every existing command onto the new core with no behavior change for current users. Then US1, US2 and US3.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: The user story the task belongs to (US1-US6 from spec.md)
- Paths are relative to the repository root. Research decisions are cited as R1-R13 and findings as F1-F10 (both in research.md).

---

## Phase 1: Setup

**Purpose**: Confirm a green baseline and include the new packages in coverage.

- [X] T001 Run `make test` and `make lint` on the unmodified branch. If anything fails, stop and report it before changing code, so new failures can be told apart from existing ones.
- [X] T002 [P] Add the `internal/environment` and `internal/fsutil` packages to `COVER_PKGS` in `Makefile`, using the same format as the existing entries.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Domain types, resolution rules, config v2, AWS listing and tagging, the multi-host SSH block, the per-instance mount, and the CLI session helper. The build must stay green after every task. Old functions that the CLI still calls are removed in T033, once nothing uses them.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T003 [P] Write tests in `internal/fsutil/atomic_test.go` for `WriteFileAtomic`. Cover these cases:
  - It creates the file with the requested mode.
  - It replaces existing content.
  - It leaves no temp files in the directory.
  - When the path is a symlink, the target is updated and the path stays a symlink.
  - When the directory is not writable, it returns an error and leaves the original file intact.
- [X] T004 Implement `WriteFileAtomic(path string, data []byte, perm os.FileMode) error` in `internal/fsutil/atomic.go`:
  1. If the path exists, resolve it with `filepath.EvalSymlinks`.
  2. Call `os.CreateTemp` in the target's directory.
  3. `Chmod` to `perm`, then write, `Sync`, `Close`, and `os.Rename` over the target.
  4. Remove the temp file on any failure.
- [X] T005 [P] Write table tests in `internal/environment/environment_test.go`:
  - `ValidateName` accepts `build`, `service`, `ci2`, `perf-test` and a 20-character name.
  - It rejects an empty string, `Build`, `-x`, `x-`, `a_b`, `a.b` and a 21-character name. The rejection message must match contracts/cli.md "Invalid name".
  - `SSHAliases()` returns `["clouddesktop-main","clouddesktop"]` for `main` and `["clouddesktop-build"]` for `build`.
  - `Names()` returns sorted names.
- [X] T006 Implement `internal/environment/environment.go`:
  - The `Environment` struct with fields `Name`, `InstanceID`, `State`, `InstanceType`, `RootVolumeID`, `RootVolumeSizeGB int32`, `PrivateIP`, `LaunchTime *time.Time` and `Legacy bool` (data-model.md).
  - Constants `LegacyName = "main"`, `MaxNameLength = 20`, `SSHAliasPrefix = "clouddesktop-"` and `LegacySSHAlias = "clouddesktop"`.
  - `ValidateName(name string) error`, using the developer-name pattern `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` plus the length limit.
  - `SSHAlias(name string) string`, the method `(Environment) SSHAliases() []string`, and `Names([]Environment) []string` (sorted).
  - A package doc comment.
- [X] T007 [P] Write table tests in `internal/environment/resolve_test.go` covering every row of the research.md R5 table:
  - An explicit name that is found, and one that is not.
  - Zero environments.
  - Exactly one environment, with and without a default, for both operations.
  - More than one environment, for each of: destroy; default set and present; default set and missing; default unset.
  - Errors must be `*ResolveError` with the right `Kind`. `Error()` must match the two-line message and remediation in contracts/cli.md "Error messages", with `Available` names sorted.
- [X] T008 Implement `internal/environment/resolve.go`:
  - `type Operation int`, with `OperationGeneral` and `OperationDestroy`.
  - `type ResolveRequest struct { Name string; Environments []Environment; Default string; Operation Operation }`.
  - `func Resolve(req ResolveRequest) (Environment, error)`, implementing R5 exactly.
  - `type ResolveError struct { Kind ResolveErrorKind; Name string; Available []string }`, with kinds `NoEnvironments`, `UnknownEnvironment`, `NameRequired`, `NoDefault` and `DefaultMissing`. `Error()` returns the problem line, a newline, and the remediation line from contracts/cli.md.
- [X] T009 [P] Extend `internal/config/config_test.go`:
  - A v2 YAML file with `default_environment` round-trips through `SaveTo` and `LoadFrom`.
  - `Validate` rejects an invalid `default_environment` and accepts an empty one.
  - `SaveTo` writes mode 0600 and leaves no temp files (it now goes through `fsutil.WriteFileAtomic`).
  - Add `TestUpdate` for the read-modify-write helper using a temp path.
- [X] T010 [P] Write tests in `internal/config/lock_test.go`:
  - `withLockAt` on a temp path serializes two goroutines: the second callback starts only after the first returns, checked with channels and timestamps.
  - The lock file is created with mode 0600.
  - The lock is released when the callback returns an error.
- [X] T011 Update `internal/config/config.go` to schema v2 (contracts/config-schema.md):
  - Add `DefaultEnvironment string` with tag `yaml:"default_environment,omitempty"`. In `Validate`, check it with `environment.ValidateName` when it is non-empty.
  - Change the `InstanceType` doc comment to "default machine size for new environments".
  - Mark `InstanceID` and `RootVolumeSizeGB` with the comment "v1 only: read for migration, cleared before saving".
  - Make `SaveTo` write through `fsutil.WriteFileAtomic` with mode 0600 (directory 0700 as today).
  - Add unexported `loadFromPath(path string) (*Config, []byte, error)`, returning the parsed config and the raw bytes, for use by migration and `Update`.
- [X] T012 Implement `internal/config/lock.go`:
  - `WithLock(fn func() error) error`, plus the testable `withLockAt(lockPath string, fn func() error) error`. It takes an exclusive `syscall.Flock` on `~/.clouddesktop/.lock` (created 0600, parent directory 0700) and unlocks and closes in a `defer`.
  - `Update(mutate func(*Config) error) error`, which takes the lock, loads with `loadFromPath`, applies `mutate`, and calls `SaveTo`.
  - Document that the lock is not reentrant: nothing called inside `WithLock` may call `WithLock` or `Load` again.
- [X] T013 [P] Extend `internal/aws/ec2_test.go`. First add a `CreateTags` method to `mockEC2API` and make `DescribeInstances` return scripted pages. Then test:
  - `ListEnvironments` sends filters `tag:Developer=<dev>`, `tag:Project=clouddesktop` and `instance-state-name` in pending, running, stopping and stopped.
  - It follows `NextToken` across pages.
  - It maps the `CloudDesktopEnv` tag to `Name`. An instance without the tag gets `Name: "main"` and `Legacy: true`.
  - It takes the root volume ID from the block device mapping that matches `RootDeviceName`.
  - It returns environments sorted by name.
  - Two instances with the same name produce an error containing both instance IDs (contracts/cli.md "Duplicate tag").
  - `TagEnvironment` calls `CreateTags` on the instance and its root volume with `CloudDesktopEnv`.
  - `RunInstance` tags the instance and volume with `CloudDesktopEnv` and uses Name `clouddesktop-<dev>-<env>` (and `-root` for the volume). `VolumeSize` comes from the params, defaulting to 100 when 0.
  - `ImportSSHKeyPair` treats an `InvalidKeyPair.Duplicate` error as success.
  - `RunInstance` and `StartInstance` turn `VcpuLimitExceeded`, `InstanceLimitExceeded` and `InsufficientInstanceCapacity` API errors into `*AWSError` with type `ErrorTypeCapacity`, and `Error()` contains both lines from the matching contracts/cli.md error row (R13, FR-004).
- [X] T014 Implement in `internal/aws/ec2.go`. Keep `FindInstanceByDeveloper` until T016.
  - Add `CreateTags` to the `ec2api` interface and add the constant `tagEnvironment = "CloudDesktopEnv"`.
  - `ListEnvironments(ctx, developerName string) ([]environment.Environment, error)`, using `ec2.NewDescribeInstancesPaginator`.
  - `TagEnvironment(ctx, instanceID, rootVolumeID, name string) error`.
  - Add `EnvironmentName` and `RootVolumeSizeGB int32` to `RunInstanceParams`, and apply the tag and volume-size changes from T013.
  - Make `ImportSSHKeyPair` tolerate duplicates.
  - Add `ErrorTypeCapacity` to `internal/aws/types.go`. In `RunInstance` and `StartInstance`, detect the three codes with `strings.Contains` on the error message, as `isTransientError` does, and return the capacity error. `main.go` prints only `Error()`, so put the cause and the next step in the message itself (R13).
- [X] T015 [P] Update `internal/aws/provisioner_test.go`:
  - Replace `TestProvision_RecoveredInstance` with two tests: an instance tagged with the requested environment is recovered; an instance tagged with a different environment is ignored and a new one is launched with `EnvironmentName` and `RootVolumeSizeGB`. Also check that an untagged legacy instance is recovered when the request is for `main`.
  - Replace `TestProvision_RunInstanceFailure_CleansUpKeyPair` with a test that `DeleteKeyPair` is not called when the launch fails (R7).
  - Replace `TestDestroy_Success` with two tests: with `deleteKeyPair=false`, `Destroy` terminates and calls neither `DeleteKeyPair`, any S3 call, nor SSM `DeleteParameter`; with `deleteKeyPair=true`, it also deletes the key pair. The bucket is never deleted (R9, FR-012).
- [X] T016 Update `internal/aws/provisioner.go`:
  - Add `EnvironmentName` and `RootVolumeSizeGB` to `ProvisionParams`. Do recovery through `ListEnvironments`, matching the name.
  - Pass both fields to `RunInstance`, and remove the key pair deletion on launch failure.
  - Change the signature to `Destroy(ctx, instanceID, developerName string, deleteKeyPair bool) error`. It terminates, deletes the key pair only when `deleteKeyPair` is true, and never touches the bucket or its parameter.
  - Delete `FindInstanceByDeveloper` and its tests from `internal/aws/ec2.go` and `internal/aws/ec2_test.go`.
  - Update the call sites so they compile and behave correctly in today's single-environment world. In `internal/cli/up.go` `runProvision`, pass `EnvironmentName: environment.LegacyName` and `RootVolumeSizeGB: 100`. In `internal/cli/destroy.go`, pass `deleteKeyPair: true`, which is correct while only one environment can exist. T030 replaces it with a computed value.
- [X] T017 [P] Extend `internal/aws/sshconfig_test.go`:
  - `renderManagedBlock` for `[main, build]` produces the exact block in contracts/external-state.md: sorted entries, `Host clouddesktop-main clouddesktop`, and a blank line between entries.
  - `syncSSHConfigAt`:
    - with environments, writes the proxy script and block and leaves content outside the markers byte-identical;
    - with zero environments, removes the block and the proxy script;
    - writes through a symlinked config and keeps the symlink;
    - writes the backup file before modifying.
  - `hostDefinedOutsideBlockAt` matches `Host foo clouddesktop-build` outside the block, ignores the same token inside the block, and does not match `clouddesktop-buildx`.
- [X] T018 Implement in `internal/aws/sshconfig.go`. Keep `WriteSSHConfig` and `RemoveSSHConfig` until T033.
  - `type SSHSettings struct { User, IdentityFile, AWSProfile, Region string }`.
  - `renderManagedBlock(envs []environment.Environment, settings SSHSettings, proxyScriptPath string) string`.
  - Exported `SyncSSHConfig(envs []environment.Environment, settings SSHSettings) error`, plus the testable `syncSSHConfigAt(sshDir, configPath, scriptPath string, deps SSMProxyDeps, envs, settings) error`. Both write through `fsutil.WriteFileAtomic`.
  - `HostDefinedOutsideBlock(alias string) (bool, error)`, plus the testable `hostDefinedOutsideBlockAt(configPath, alias string)`, which compares whitespace-separated tokens on `Host` lines outside the markers.
- [X] T019 [P] Extend `internal/aws/ssm_test.go` and `internal/aws/provisioner_test.go`:
  - The `RunS3Mount` script:
    - exits non-zero after writing `bootstrap in progress` to stderr when `/var/log/bootstrap-system-complete` is missing;
    - exits 0 before any `umount` when `mountpoint -q /home/ubuntu/s3`, `timeout 10 ls /home/ubuntu/s3` and `grep -q '^<bucket> /home/ubuntu/s3' /etc/fstab` all succeed;
    - still writes the fstab entry otherwise.
  - `WaitUntilCommandComplete` includes the invocation's trimmed `StandardErrorContent` in the error for a Failed status.
  - `RunS3Mount` and `RunFilesystemExtension` retry `SendCommand` while it fails with `InvalidInstanceId`, succeed once it stops failing, and give up after the overall limit with an error saying SSM can't reach the instance yet. Use short intervals in tests.
  - `Provisioner.EnsureS3Mount`:
    - creates the bucket and parameter when missing;
    - recreates them when the parameter points at a deleted bucket;
    - always sends the mount command to the given instance, including when the bucket already exists. That last case is the F5 regression test.
- [X] T020 Implement the mount changes. Keep `SetupS3Mount` until T033.
  - Add the guard and healthy-mount short-circuit from T019 to the `RunS3Mount` script in `internal/aws/ssm.go`. The guard writes its message to stderr.
  - Add an unexported `sendCommand` helper in `internal/aws/ssm.go` that retries `SendCommand` on `InvalidInstanceId` with exponential backoff (2 s initial, 15 s max, 3-minute overall limit). Match the code with `strings.Contains`, like the package's other error helpers. Use the helper from `RunS3Mount` and `RunFilesystemExtension`, and make the intervals fields on `SSMClient` so tests can shorten them, as `EC2Client` does (research R8). On the first retry, print `Waiting for SSM to reach the instance...` once, so a long wait doesn't look like a hang.
  - In `WaitUntilCommandComplete`, add `StandardErrorContent` to failure errors.
  - Add `Provisioner.EnsureS3Mount(ctx, instanceID, developerName string) error` to `internal/aws/provisioner.go`. It moves the bucket-exists and recreate logic from `ensureS3Mount` in `internal/cli/up.go` into the provisioner, then always runs `RunS3Mount` and waits (R8).
- [X] T021 Implement `internal/cli/target.go`:
  - `type session struct { cfg *config.Config; ec2 *vfaws.EC2Client; ssm *vfaws.SSMClient; s3 *vfaws.S3Client; envs []environment.Environment }`.
  - `openSession(ctx) (*session, error)`: run `config.Load`, `vfaws.ValidateSession`, create the three clients and call `ListEnvironments`. For each `Legacy` environment, call `TagEnvironment` best-effort and ignore its error (R3).
  - `(s *session) resolve(name string, op environment.Operation) (environment.Environment, error)`, which calls `environment.Resolve` with `s.cfg.DefaultEnvironment`.
  - `printTarget(env environment.Environment)`, which prints `Environment: <name> (<instance-id>)`.
  - `(s *session) syncSSHConfig(ctx) error`. Inside `config.WithLock`, it re-lists environments and calls `vfaws.SyncSSHConfig` with `User: "ubuntu"` and the config's key, profile and region (R6, R7).
  - `nameArg(args []string) string`.

**Checkpoint**: `go build ./... && make test` pass. Commands behave exactly as before.

---

## Phase 3: User Story 4 - Existing users keep working without re-provisioning (Priority: P1)

**Goal**: Migrate v1 configs. Every existing command resolves its target from the AWS listing and accepts an optional `[name]`. The legacy instance becomes `main`, the `clouddesktop` alias keeps working, and the S3 mount is ensured per instance.

**Independent Test**: quickstart.md section 2 against an environment created by the previous release.

- [X] T022 [P] [US4] Write tests in `internal/config/migrate_test.go`:
  - A v1 file becomes v2: `default_environment: main`, no `instance_id` or `root_volume_size_gb`, all other fields preserved.
  - `config.yaml.v1-backup` is byte-identical to the original, with mode 0600.
  - An existing backup is not overwritten.
  - A second load is a no-op: no rewrite and no new backup.
  - When the directory is read-only, the original v1 file is untouched and an error is returned (FR-014).
  - A v2 file is never rewritten.
- [X] T023 [US4] Implement `internal/config/migrate.go` and wire it into `LoadFrom` in `internal/config/config.go`, following contracts/config-schema.md.
  - A config is legacy when `InstanceID != ""`.
  - Under the lock at the config's directory: write the backup if absent; set `DefaultEnvironment = environment.LegacyName`; clear `InstanceID` and `RootVolumeSizeGB`; save atomically.
  - Print the one-time notice to stderr, then validate.
- [X] T024 [P] [US4] Convert `internal/cli/status.go`:
  - `Use: "status [name]"` with `Args: cobra.MaximumNArgs(1)`.
  - Use `openSession`, `resolve(nameArg(args), environment.OperationGeneral)` and `printTarget`, and describe the resolved instance ID. Remove every `cfg.InstanceID` reference.
- [X] T025 [P] [US4] Convert `internal/cli/down.go` the same way (`down [name]`). Stop and wait on the resolved instance and keep the current messages. The `--all` flag comes in T052.
- [X] T026 [P] [US4] Convert `internal/cli/ssh.go` (`ssh [name]`): `openSession`, `resolve`, `printTarget`, then `syscall.Exec` of `ssh <environment.SSHAlias(env.Name)>`.
- [X] T027 [P] [US4] Convert `internal/cli/resize.go` (`resize-instance [name]`):
  - Resolve the target and call `printTarget` before the interactive select (FR-023).
  - Compare the selection against `env.InstanceType` (the actual type), not `cfg.InstanceType`.
  - Remove the `config.Save` of `InstanceType`. Config `instance_type` is now only the default for new environments (F6).
- [X] T028 [P] [US4] Convert `internal/cli/resize_storage.go` (`resize-storage [name]`): resolve the target, call `printTarget` before any prompt (FR-023), and remove the `RootVolumeSizeGB` config write (F6).
- [X] T029 [US4] Convert `internal/cli/up.go` (`up [name]`):
  - Use `openSession`.
  - When `resolve` fails with kind `NoEnvironments` and no name was given, provision `main` through the existing `runProvision` path. Then call `config.Update` to set `DefaultEnvironment` when it is empty.
  - Otherwise start the resolved environment as today, with `printTarget` first.
  - Replace `writeSSHConfig` with `s.syncSSHConfig`.
  - Print `Setting up S3 mount on <name>...`, then replace `ensureS3Mount` with `provisioner.EnsureS3Mount(ctx, env.InstanceID, cfg.DeveloperName)`, keeping today's warning-on-failure text. This fixes F5.
  - Make `printInstanceSummary` show `Environment: <name>` and `SSH: ssh clouddesktop-<name>`.
  - Remove the `cfg.InstanceID` writes.
- [X] T030 [US4] Convert `internal/cli/destroy.go` (`destroy [name] --confirm`):
  - Move the `--confirm` check out of `RunE` and into `runDestroy`, so the warning is always shown first.
  - Resolve with `environment.OperationDestroy`, then `printTarget`.
  - Print the FR-011 warning exactly as in contracts/cli.md: the environment name; "Your other environments are not affected: <names or 'none'>"; "Your personal bucket ... is kept ... s3-reset".
  - Without `--confirm`, stop here and return the contract error `re-run with --confirm to destroy environment "<name>"`, changing nothing (FR-011).
  - Immediately before destroying, call `ListEnvironments` again and set `deleteKeyPair` to true only when no environment other than the target remains (R9). Then call `provisioner.Destroy(..., deleteKeyPair)`.
  - Call `s.syncSSHConfig`, which removes the block when no environments remain.
  - If the destroyed environment was the default, use `config.Update` to clear `DefaultEnvironment`.
  - Keep `config.yaml`: delete `cleanupConfig` and its call. The final message is `Environment "<name>" destroyed.` (R9).
- [X] T031 [P] [US4] Convert `internal/cli/s3_reset.go`. Use `openSession`, and move the `--confirm` check from `RunE` into the run function. Before deleting, print the bucket name and `Environments that use it: <name> (<state>), ...` (or "none"). Without `--confirm`, stop there and return the contract error `re-run with --confirm to delete bucket <bucket>`, changing nothing. After deleting, print the repair note from contracts/cli.md (FR-012). The delete logic is unchanged.
- [X] T032 [US4] Update the refusal in `internal/cli/init.go`. When an existing config loads, list environments with its profile, region and developer name. If any exist, refuse with `clouddesktop is already initialized and you have environments: <names>` followed by the remediation `Destroy them first if you need to change your developer name or SSH key.` (FR-021). Replace the `existing.InstanceID` check.
- [X] T033 [US4] Remove the code the converted commands no longer use:
  - `WriteSSHConfig`, `RemoveSSHConfig` and `SSHConfigEntry` in `internal/aws/sshconfig.go`, and their tests;
  - `Provisioner.SetupS3Mount`;
  - `ensureS3Mount` and `writeSSHConfig` in `internal/cli/up.go`.

  Then confirm that `grep -rn "InstanceID\b" internal/cli` finds only `env.InstanceID`/`info.InstanceID` uses and that `cfg.InstanceID` appears only in `internal/config`. Run `go build ./... && make test && make lint`.
- [ ] T034 [US4] Run quickstart.md section 2 and confirm every expected outcome.

**Checkpoint**: Existing single-environment users work unchanged on the new core. The bucket is kept on destroy, and the mount is ensured per instance.

---

## Phase 4: User Story 1 - Create an additional environment (Priority: P1)

**Goal**: `clouddesktop create <name>` launches an additional environment without touching existing ones.

**Independent Test**: quickstart.md section 3.

- [X] T035 [P] [US1] Add tests to `internal/environment/environment_test.go` for `ValidateNew(name string, existing []Environment) error`. It fails for an invalid name, using the `ValidateName` message, and for a taken name, using contracts/cli.md "Name taken". It succeeds otherwise.
- [X] T036 [P] [US1] Add tests to `internal/config/config_test.go` for the exported `IsValidInstanceType` and `IsValidStorageSize`: every entry in `ValidInstanceTypes` and `ValidStorageSizes` is accepted, and `m5.large` and `150` are rejected.
- [X] T037 [US1] Implement `ValidateNew` in `internal/environment/environment.go`. Export `IsValidInstanceType`, by renaming `isValidInstanceType` and updating its callers, and add `IsValidStorageSize` in `internal/config/config.go`.
- [X] T038 [US1] Implement `internal/cli/create.go`.
  - Command shape:
    - `NewCreateCmd()` with `Use: "create <name>"` and `Args: cobra.ExactArgs(1)`.
    - Flags `--instance-type` (default: `cfg.InstanceType`, applied at run time) and `--storage-size` (default 100).
    - `Long` help states the naming convention from contracts/cli.md.
  - Run flow:
    1. `openSession`, then `ValidateNew`. Validate the type and size, listing valid values on error.
    2. Run the `vfaws.HostDefinedOutsideBlock(environment.SSHAlias(name))` collision check.
    3. Call `provisionEnvironment(ctx, s, name, instanceType, storageGB)`, a function in this file that holds the logic moved from `runProvision` in `internal/cli/up.go`. It prints `Environment: <name> (creating)` before calling Provision, prints the full target line once the instance ID is known (FR-023), and waits until running. If the wait fails, it terminates the instance and returns the error (FR-017). It then calls `s.syncSSHConfig`.
    4. If the developer had zero environments before, call `config.Update` to set `DefaultEnvironment` to the new name.
    5. Print the summary and `Run 'clouddesktop up <name>' after bootstrap (~10 minutes) to mount your personal bucket at /home/ubuntu/s3`.
- [X] T039 [US1] In `internal/cli/up.go`, replace the zero-environment `runProvision` path with `provisionEnvironment(ctx, s, environment.LegacyName, cfg.InstanceType, 100)` and delete `runProvision`. Register `NewCreateCmd()` in `internal/cli/root.go`.
- [ ] T040 [US1] Run quickstart.md section 3 and confirm every expected outcome.

**Checkpoint**: A developer can own two environments. Each is reachable by its alias and targetable by name, and destroying one leaves the other intact.

---

## Phase 5: User Story 2 - See all my environments at a glance (Priority: P1)

**Goal**: `clouddesktop list` / `ls` shows every environment, the default, and the bucket.

**Independent Test**: quickstart.md section 4.

- [X] T041 [P] [US2] Add tests to `internal/aws/ec2_test.go` for `RootVolumeSizes(ctx, volumeIDs []string) (map[string]int32, error)`: one `DescribeVolumes` call for all IDs, the correct mapping, and no call for empty input. Also assert that for 10 environments, `ListEnvironments` plus `RootVolumeSizes` make exactly one DescribeInstances call per result page and one DescribeVolumes call, so the cost of `list` doesn't grow with the number of environments (SC-004, R10).
- [X] T042 [P] [US2] Write tests in `internal/cli/list_test.go` for `renderEnvironmentTable(w io.Writer, envs []environment.Environment, defaultName string)`. It must render the exact header and rows from contracts/cli.md, sorted by name with `*` on the default. When `defaultName` matches no environment, it adds the notice line `Default environment "<name>" no longer exists. Set one with 'clouddesktop use <name>'.`.
- [X] T043 [US2] Implement `RootVolumeSizes` in `internal/aws/ec2.go`.
- [X] T044 [US2] Implement `internal/cli/list.go`:
  - `NewListCmd()` with `Use: "list"`, `Aliases: []string{"ls"}` and `Args: cobra.NoArgs`.
  - Flow: `openSession`, then `RootVolumeSizes` for all root volumes, then `renderEnvironmentTable` (using `text/tabwriter`).
  - With zero environments, print the zero-environment message.
  - Print the bucket line: `GetDeveloperS3Bucket` + `BucketExists`, with the wording from contracts/cli.md for both cases. List must never start or modify anything.
  - Register the command in `internal/cli/root.go`.
- [ ] T045 [US2] Run quickstart.md section 4, including the time under 10 s. The 10-environment target in SC-004 is covered by the call-count test in T041.

**Checkpoint**: Developers can see every environment and the default in one command.

---

## Phase 6: User Story 3 - Target a specific environment, or rely on my default (Priority: P1)

**Goal**: Set and switch the default, and show it in status. Name resolution, the destroy name requirement and the target line already exist from Phases 2-3.

**Independent Test**: quickstart.md sections 5 and 7. It needs two environments, from US1.

- [X] T046 [P] [US3] Implement `internal/cli/use.go`:
  - `NewUseCmd()` with `Use: "use <name>"` and `Args: cobra.ExactArgs(1)`.
  - Flow: `openSession`, then `resolve(name, OperationGeneral)`, which fails with the unknown-name error. Then `config.Update` sets `DefaultEnvironment`, and the command prints `Default environment: <name>`.
  - Register the command in `internal/cli/root.go`.
- [X] T047 [P] [US3] In `internal/cli/status.go`, add a `Default: yes|no` line after the target line, comparing with `cfg.DefaultEnvironment`.
- [ ] T048 [US3] Run quickstart.md sections 5 and 7 and confirm every expected outcome, including that `destroy` without a name is refused when there are two environments.

**Checkpoint**: All P1 stories (US1-US4) are complete. This is the minimum releasable scope.

---

## Phase 7: User Story 5 - Recover access on a new machine (Priority: P2)

**Goal**: `init` finds existing environments, asks for the default, and restores aliases.

**Independent Test**: quickstart.md section 8.

- [X] T049 [US5] Extend `internal/cli/init.go` so discovery happens right after the developer name and AWS profile are chosen (contracts/cli.md `init` row):
  1. Call `vfaws.ValidateSession`, create an EC2 client, and call `ListEnvironments`. If listing fails, print `Warning: could not look up existing environments: <err>` and continue as if none were found.
  2. When environments are found, print `Found N environment(s): <names>` and `Select the SSH key these environments were created with.` before the SSH key prompt.
  3. Run the remaining prompts (instance type, SSH key) as today.
  4. With 1 environment, set `DefaultEnvironment` to it. With more than 1, show a `huh.NewSelect` titled `Default environment` whose options are the names. With 0, do nothing.

  After saving, when environments were found, call `vfaws.SyncSSHConfig` so the aliases work immediately. The final hint changes: `Run 'clouddesktop list'` when environments exist, otherwise the current `Run 'clouddesktop up'`. Environments of other developers are excluded by the `Developer` tag filter (FR-015).
- [ ] T050 [US5] Run quickstart.md section 8 and confirm every expected outcome.

---

## Phase 8: User Story 6 - Stop all my environments at once (Priority: P3)

**Goal**: `clouddesktop down --all`.

**Independent Test**: quickstart.md section 9.

- [X] T051 [P] [US6] Write tests in `internal/cli/down_test.go` for `stopAll(ctx context.Context, ec2 vfaws.EC2Service, envs []environment.Environment, out io.Writer) error`, using `testing.MockEC2Client` from `internal/testing/mock.go`:
  - It stops only running and pending environments and skips stopped ones.
  - It calls `StopInstance` for every target before waiting on any.
  - It continues after one `StopInstance` failure, prints one line per environment, and returns an error naming the failed environments.
  - With no running or pending environments, it prints `No running environments.` and returns nil.
- [X] T052 [US6] Implement `stopAll` and the `--all` flag in `internal/cli/down.go`:
  - `--all` cannot be combined with a name; use the contract error `--all cannot be combined with an environment name`.
  - With `--all`, the command skips `resolve` and calls `stopAll` with `s.envs`.
  - The command exits non-zero when any stop fails (FR-019).
  - With nothing running or pending, `--all` prints `No running environments.` and exits 0.
- [ ] T053 [US6] Run quickstart.md section 9, including the concurrent `up` check for FR-016.

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T054 [P] Update `README.md`:
  - Commands reference: add `create`, `list`/`ls` and `use`, `[name]` on every environment command, and `down --all`.
  - The naming convention.
  - Multiple environments and the default.
  - Destroy behavior changes: the bucket and config are kept, and the bucket is deleted with `s3-reset --confirm`.
  - The v1 migration and its backup file.
  - Bucket mount per environment after bootstrap.
  - A note for administrators to activate `Developer` and `CloudDesktopEnv` as cost-allocation tags (contracts/external-state.md).
- [X] T055 [P] Update the project `CLAUDE.md`:
  - The `internal/environment` and `internal/fsutil` packages.
  - Tags as the environment registry (`CloudDesktopEnv`).
  - Config v2 and migration.
  - The multi-host SSH block.
  - Correct `scripts/bootstrap.sh` to `scripts/bootstrap-system.sh`.
- [X] T056 Run `gofmt -l .` (expect no output), `make test`, `make lint` and `make build-all`. Then run `grep -rnE "TODO|FIXME" internal cmd` and expect no new hits.
- [ ] T057 Run quickstart.md sections 6, 10 and 11 (mount on every environment, per-environment idle auto-stop for FR-018, then full cleanup) and confirm nothing remains in the test account for the QA developer name.

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)**: none.
- **Foundational (Phase 2)**: needs Setup, and blocks every story.
- **US4 (Phase 3)**: needs Foundational, and blocks US1, US2, US3, US5 and US6, because it moves the commands onto the session helper.
- **US1 (Phase 4)**: needs US4.
- **US2 (Phase 5)**: needs US4. It can run in parallel with US1 apart from the `root.go` edit.
- **US3 (Phase 6)**: needs US4. Its independent test needs two environments, which come from US1.
- **US5 (Phase 7)** and **US6 (Phase 8)**: need US4, and are independent of each other.
- **Polish (Phase 9)**: after all desired stories.

### Within Phase 2

T004 needs T003. T006 needs T005. T008 needs T006 and T007. T011 needs T004, T006 and T009. T012 needs T011 and T010. T014 needs T006 and T013. T016 needs T014 and T015. T018 needs T004, T006 and T017. T020 needs T016 and T019. T021 needs every Phase 2 implementation task.

### Shared files (serialize these edits)

| File | Tasks |
|------|-------|
| `internal/cli/root.go` | T039, T044, T046 |
| `internal/cli/status.go` | T024, then T047 |
| `internal/cli/down.go` | T025, then T052 |
| `internal/cli/up.go` | T016, then T029, then T033, then T039 |
| `internal/cli/destroy.go` | T016, then T030 |
| `internal/cli/init.go` | T032, then T049 |

### Within each story

Tests come first and must fail, then implementation, then the quickstart run.

---

## Parallel Examples

```text
# Phase 2: all test-writing tasks touch different files
T003 internal/fsutil/atomic_test.go
T005 internal/environment/environment_test.go
T007 internal/environment/resolve_test.go
T009 internal/config/config_test.go
T010 internal/config/lock_test.go
T013 internal/aws/ec2_test.go
T015 internal/aws/provisioner_test.go
T017 internal/aws/sshconfig_test.go

# US4: command conversions after T021 and T023
T024 status.go   T025 down.go   T026 ssh.go   T027 resize.go   T028 resize_storage.go   T031 s3_reset.go

# After US4: separate stories in parallel
US1 (T035-T040)   US2 (T041-T045, root.go edit serialized)   US6 (T051-T053)
```

---

## Implementation Strategy

### MVP

Phases 1-4 (Setup, Foundational, US4, US1) are the smallest demonstrable slice: existing users are unaffected, and a developer can create and use a second environment by name. Do not release it to users without US2 and US3, because all four P1 stories make up the releasable scope (spec priorities).

### Incremental delivery

1. Complete Phases 1-3, then run quickstart section 2. This gives zero regression for every current user and fixes F5.
2. Add US1, then US2, then US3, running each story's quickstart section. Release after US3.
3. Add US5 and US6 as follow-up increments.
4. Finish with Polish (docs and final validation).

### Commit guidance

Commit after each task or each test-plus-implementation pair, with scope labels per the repo's PR conventions. The build and `make test` must pass at every commit.

---

## Notes

- **Out of scope** (research R12): cross-developer S3 access through the instance role, and self-asserted developer identity. Raise separate tickets.
- **Follow-up** (research R8): mounting the bucket at bootstrap, to remove the second `up` after `create`.
- **FR-018 needs no code**: the idle check in `scripts/bootstrap-system.sh` already runs on each instance independently. Quickstart section 10 verifies it (T057).
