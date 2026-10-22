# Research: Multiple Cloud Desktops per Developer

Phase 0 output for [plan.md](./plan.md). Each decision records what was chosen, why, and what else was considered.

**Review (2026-10-21)**: The developer confirmed R1 (tags as the source of truth), R2 (`CloudDesktopEnv` key), R4 (positional names and a separate `create`) and R8 (mount ensured on `up`; a new environment needs one `up` after bootstrap). The environment name is its function (for example `build` or `service`); there is no separate function field. The convention is a one-word function name. Hyphens stay allowed, and FR-002 is unchanged.

## Findings in the current code that shape the design

| # | Finding | Where | Impact |
|---|---------|-------|--------|
| F1 | Config stores a single `instance_id`, and every command reads it | `internal/config/config.go:16` | Replaced by name resolution (R1, R5) |
| F2 | Provision "recovers" the first instance tagged with the developer before creating one | `internal/aws/ec2.go:152`, `internal/aws/provisioner.go:44` | `create build` would adopt `main` instead of launching. Recovery must match developer and environment. |
| F3 | Destroy deletes the key pair, the bucket, its SSM parameter, the SSH block and the whole config file | `internal/aws/provisioner.go:109`, `internal/cli/destroy.go:78` | Would break other environments and contradicts FR-010 and FR-012 (R9) |
| F4 | The SSH managed block holds one `Host clouddesktop` entry | `internal/aws/sshconfig.go:186`, `internal/cli/up.go:224` | Becomes a multi-host block (R6) |
| F5 | Freshly provisioned instances never get the S3 mount. `ensureS3Mount` returns early when the SSM parameter and the bucket exist, which is always true after provisioning, and bootstrap installs `mount-s3` without mounting. | `internal/cli/up.go:189`, `scripts/bootstrap-system.sh:184` | Existing bug, found by reading the code and not yet reproduced. With several environments, every environment after the first would stay unmounted (R8). |
| F6 | `resize-instance` compares against and writes `cfg.InstanceType`; `resize-storage` writes `cfg.RootVolumeSizeGB` | `internal/cli/resize.go:56,98`, `internal/cli/resize_storage.go:121` | These become per-environment facts read from AWS, not config |
| F7 | CLI commands construct concrete AWS clients, and `internal/cli` has no tests | `internal/cli/*.go` | New logic goes in testable packages (R11) |
| F8 | The SSM proxy script is environment-independent: it receives the instance ID as `%h` | `internal/aws/sshconfig.go` (`renderProxyScript`) | Reused unchanged for every environment |
| F9 | Instance metadata tags are enabled, and bootstrap writes `/var/log/bootstrap-system-complete` when done | `internal/aws/ec2.go:322`, `scripts/bootstrap-system.sh:327` | Used by the mount guard (R8) |
| F10 | The constitution is the unfilled template | `.specify/memory/constitution.md` | No ratified gates; CLAUDE.md standards are used instead (plan.md) |

## R1. Source of truth for which environments exist

**Decision**: AWS instance tags. Each instance carries `CloudDesktopEnv=<name>` next to the existing `Developer`, `Project` and `ManagedBy` tags. The local config holds no per-environment records, only developer settings and `default_environment`. Every command lists the developer's environments with one tag-filtered DescribeInstances call and resolves its target from that list.

**Rationale**:
- Local state can't drift from the cloud. There are no stale records (FR-008), no separate discovery step on a new laptop (FR-015 is the normal code path), and no registry to migrate.
- Commands against different environments never write per-environment local state, which removes most of FR-016's surface.
- Every command already makes an EC2 call (DescribeInstance) before acting. Replacing it with a filtered DescribeInstances keeps the call count the same.

**Alternatives considered**:
- *Local registry (name to instance ID map in config.yaml)*: gives offline name resolution and no AWS call for `ssh`. It needs discovery, stale-record handling, reconciliation and locking on every environment write. Rejected because it adds more states with no user-visible gain, and `ssh` already needs AWS credentials for the SSM proxy.
- *Local cache plus tags*: two sources to reconcile. Rejected.

**Consequence for the spec** (applied 2026-10-21): FR-008, US2 scenario 2, the matching edge case and the Local Settings entity now say that an environment deleted outside the tool drops out of `list`. Only a default that points at a missing environment is flagged, and `list` and resolution both say so.

## R2. Tag key for the environment name

**Decision**: `CloudDesktopEnv`, applied to the instance and its root volume at RunInstances. New Name tags are `clouddesktop-<developer>-<env>` and `clouddesktop-<developer>-<env>-root`.

**Rationale**: `Environment` is a common organization-wide key (prod, staging, dev). An AWS Organizations tag policy that restricts its values would make RunInstances fail, and cost reports would mix two meanings. A namespaced key avoids both and matches the existing PascalCase keys.

**Alternatives considered**: `Environment` (collision risk); parsing the `Name` tag (fragile, and legacy Name tags have no environment suffix).

## R3. Migrating existing single-environment users

**Decision**:
1. **Config file**: `config.Load` detects a v1 file (top-level `instance_id` present). It writes a byte-for-byte backup to `~/.clouddesktop/config.yaml.v1-backup` (0600), sets `default_environment: main`, drops `instance_id` and `root_volume_size_gb`, and atomically replaces the file. No AWS call is involved.
2. **Legacy instance**: an instance tagged `Project=clouddesktop` and the developer's `Developer` tag, but with no `CloudDesktopEnv` tag, is listed as `main`. The first command that sees it adds `CloudDesktopEnv=main` on a best-effort basis. If tagging fails (for example, CreateTags is denied), it is ignored, and the listing rule keeps working.
3. The legacy Name tag (`clouddesktop-<developer>`) is left unchanged.

**Rationale**: FR-013 forbids stopping or modifying the machine, and tags are metadata only. The atomic replace plus backup satisfies FR-014. The fallback rule means migration never depends on a permission we haven't confirmed, and the tag makes FR-020 reporting complete wherever it succeeds.

**Alternatives considered**: tag-only migration (fails outright if CreateTags is denied); leaving legacy instances untagged forever (cost reports would lack the environment name).

**Risks**:
- Downgrading to the previous binary with a v2 config is unsupported. The old binary sees no `instance_id` and "recovers" the first tagged instance. The backup file allows a manual restore.
- Two untagged legacy instances for one developer are only possible if created by hand. Listing fails with a duplicate-name error that names both instance IDs.

## R4. CLI surface for targeting and creation

**Decision**:
- Every environment command takes an optional positional environment name (`clouddesktop ssh build`).
- New commands: `create <name>`, `list` (alias `ls`) and `use <name>`.
- `down --all` covers FR-019.
- `up` with no name and zero environments keeps today's behavior and provisions `main`.
- `create` takes `--instance-type` (default: config `instance_type`, which now means "default for new environments") and `--storage-size` (default 100, from `ValidStorageSizes`). It doesn't prompt, so it can be scripted.

**Rationale**:
- A positional argument is the shortest form for the most frequent commands (`ssh`, `up`, `down`).
- `create` is separate because US3 scenario 3 requires an unknown name to fail without changes, so a mistyped `up biuld` must not launch a billable machine.
- `down --all` reuses an existing verb instead of adding a top-level command.

**Alternatives considered**:
- An `--env`/`-e` flag: more typing. It only pays off if commands later need other positional arguments, and Cobra's `--` passthrough works with positional arguments anyway.
- `up <new-name>` creating on demand: typos would create billable machines.
- A separate `stop-all` command: one more top-level verb.

## R5. Default environment rules

**Decision**: The default is stored as `default_environment` in config. Resolution is one pure function in `internal/environment`:

| Name given | Environments | Default | Command | Result |
|------------|--------------|---------|---------|--------|
| yes | any | any | any | That environment, or "unknown environment" listing the valid names |
| no | 0 | n/a | `up` | Provision `main` |
| no | 0 | n/a | other | "No environments" plus how to create one |
| no | 1 | any | any | The only environment |
| no | more than 1 | any | `destroy` | Error: name required |
| no | more than 1 | set, exists | other | The default |
| no | more than 1 | set, missing | other | Error: default no longer exists; run `use` |
| no | more than 1 | unset | other | Error: no default; lists names; run `use` |

The default is **set** on migration (`main`), when the first environment is created (zero to one), and by `use`. It is **cleared** when the default environment is destroyed, and never chosen automatically otherwise (FR-022). Before acting, every command prints `Environment: <name> (<instance-id>)` (FR-023).

**Rationale**: One pure function covers US3 scenarios 3-8 and FR-006, FR-022 and FR-024, and it can be tested exhaustively as a table.

**Alternatives considered**: making the bare `clouddesktop` alias follow the default (see R6).

## R6. SSH config for several environments

**Decision**:
- One managed block between the existing markers, containing one `Host` entry per non-terminated environment. It is regenerated from the full environment list every time it is written (`up`, `create`, `destroy`).
- Aliases are `clouddesktop-<env>`. Environment `main` also answers to `clouddesktop`. The shared proxy script is unchanged.
- Before `create`, the tool refuses a name whose alias already appears on a `Host` line outside the managed block.
- Writes are atomic (temp file in the same directory, then rename). If `~/.ssh/config` is a symlink, the target file is written, so dotfile managers keep working.

**Rationale**:
- Regenerating from the source of truth heals itself: stale entries disappear, and concurrent writers converge.
- One block keeps the existing marker parsing and backup behavior.
- Pinning the bare alias to `main` satisfies FR-013 and keeps remote-editor workspaces stable. An alias that silently followed the default would open a different machine in a saved VS Code remote after `use`.

**Alternatives considered**: one marker pair per environment (more parsing, and still races); a bare alias that follows the default (violates FR-013 and surprises editor users); dropping the bare alias (breaks existing users).

## R7. Concurrency and local-state writes

**Decision**:
- An advisory lock (`flock` on `~/.clouddesktop/.lock`) is held during config read-modify-write and SSH block regeneration. The environment list for the SSH block is fetched while holding the lock, and long AWS waits happen outside it.
- All file writes are atomic.
- EC2 key pair import treats "already exists" as success.
- Provision no longer deletes the key pair when a launch fails.

**Rationale**:
- FR-016 is a MUST, and without a lock a `use` racing with destroy-of-default can lose an update.
- The lock is about 20 lines using `syscall.Flock`, and darwin and linux are the only build targets.
- The key pair is shared per developer and only read at launch. Two concurrent `create`s doing delete-then-import can race, so tolerating "already exists" is safe. Deleting it on a failed launch could break a concurrent `create`.

**Alternatives considered**: no lock, relying on self-healing (violates "MUST NOT lose updates"); per-environment state files (unnecessary given R1).

## R8. Personal bucket and its mount

**Decision**:
- **Bucket lifecycle (FR-012)**: destroy never touches the bucket or its SSM parameter. `s3-reset --confirm` remains the explicit delete, and it first lists every environment that uses the bucket. Without `--confirm`, it prints the same list, changes nothing and exits non-zero.
- **Mount, ensured per instance**: on every `up` of a running instance, the CLI sends the mount command. The script:
  - exits 0 without changes when `/home/ubuntu/s3` is mounted, healthy (a time-bounded listing succeeds) and `/etc/fstab` references the right bucket;
  - exits non-zero, writing "bootstrap in progress" to stderr, when `/var/log/bootstrap-system-complete` is missing;
  - otherwise (re)mounts as it does today.
- **SSM readiness**: right after `up` starts a stopped instance, its SSM agent may not have reconnected, and `SendCommand` fails with `InvalidInstanceId`. The CLI retries with exponential backoff (2 s initial, 15 s max, 3-minute limit) before falling back to today's warning. Today's code never sends a command on a normal start, so without the retry most `up` runs from a stopped state would print a warning. The CLI prints one progress line on the first retry so the wait doesn't look like a hang.
- **`create` does not mount**, because bootstrap is still running. It prints "run `clouddesktop up <name>` after bootstrap", which matches today's first-provision message.

**Rationale**: This fixes F5 and makes mount state a per-instance fact, which several environments require. The health check also repairs mounts on other environments left stale by `s3-reset`.

**Alternatives considered**:
- *Tag the instance after mounting*: needs CreateTags, and the tag goes stale after `s3-reset`.
- *Mount during bootstrap*, reading the bucket name from an instance tag via instance metadata: removes the second `up` for new environments, but adds a second mount code path. Recommended as a follow-up.

**Cost**: one SSM command round-trip (a few seconds) on each `up`, plus up to 3 minutes of retries right after a start while the SSM agent reconnects.

## R9. Destroy semantics

**Decision**:
- Terminate only the target instance (its root volume is deleted on termination, as today).
- Delete the shared key pair only when no other environment remains, judged from a fresh listing taken immediately before destroying. This narrows, but can't fully close, the window in which a concurrent `create` loses the key pair between importing it and launching. If that happens, the launch fails with a clear error and can be retried.
- Never delete the bucket or its parameter.
- Regenerate the SSH block, and remove the block and the proxy script when no environments remain.
- Clear the default if it pointed at the destroyed environment.
- Keep `config.yaml`. Today destroy deletes it, but re-running `init` is no longer needed before creating again.
- The confirmation text follows FR-011: the environment name, the other environments that are unaffected, and that the bucket is kept, with the command to delete it. Without `--confirm`, the command prints the same warning, changes nothing and exits non-zero, so the developer sees exactly what will happen before confirming.

**Rationale**: FR-010, FR-011, FR-012 and FR-022.

**Alternatives considered**: deleting config on the last destroy (forces a re-init for no benefit).

## R10. Listing and disk sizes

**Decision**: One paginated DescribeInstances call, filtered by `tag:Developer`, `tag:Project=clouddesktop` and states pending, running, stopping and stopped. Then one DescribeVolumes call for all root volume IDs, and one bucket existence check. If two instances share an environment name, the call fails with an error naming both instance IDs.

**Rationale**: The number of API calls stays constant regardless of environment count, which meets SC-004 (10 s for 10 environments) with a wide margin. Excluding `shutting-down` lets a name be reused right after destroy.

**Alternatives considered**: calling `GetRootVolumeInfo` per instance (N calls).

## R11. Testing approach

**Decision**: Keep business logic in testable units that follow the existing patterns:
- table tests for the resolver and name validation;
- `t.TempDir()` file tests for config migration, atomic writes, the lock, and SSH block rendering and collision detection;
- EC2 API-level mocks through `newEC2ClientWithAPI` for listing, tag mapping, provision and destroy.

CLI files stay thin glue. The two pure functions extracted from commands (`renderEnvironmentTable` in `list.go`, `stopAll` in `down.go`) get unit tests in `internal/cli`; no harness for running whole commands is added. End-to-end checks are manual and follow [quickstart.md](./quickstart.md).

**Rationale**: This matches today's test style. `internal/cli` has no tests, and building a harness for it is outside this feature.

## R12. Out of scope, found during research

- The instance-role policy `clouddesktop-s3-developer-bucket` (`terraform/shared/iam.tf`) grants object read and write on `arn:aws:s3:::clouddesktop-*`. Any developer's instance can therefore read and write every other developer's bucket. This feature doesn't change that; it needs its own ticket.
- Developer identity is self-asserted (`developer_name` in local config), and tag filters don't authenticate ownership. FR-015's "never adopt another developer's environments" holds only while developer names are unique and honest. This is a pre-existing limitation.
- The project `CLAUDE.md` refers to `scripts/bootstrap.sh`, but the file is `scripts/bootstrap-system.sh`.

## R13. Capacity and quota errors

**Decision**: `RunInstance` and `StartInstance` turn the EC2 error codes `VcpuLimitExceeded`, `InstanceLimitExceeded` and `InsufficientInstanceCapacity` into an `AWSError` with a new type, `ErrorTypeCapacity`. The codes are detected by matching the error message, like the package's existing `isTransientError` and `isNotFoundError`, so no new direct dependency is added. `cmd/clouddesktop/main.go` prints only `Error()`, so the message itself names the limit and the next step (contracts/cli.md "Error messages").

**Rationale**: FR-004 requires naming the limit and what the developer can do. With no per-developer limit, the account-wide on-demand vCPU quota is the limit developers will actually hit.

**Alternatives considered**: passing the raw AWS error through (it names the code but not the fix); printing `Remediation()` from `main.go` for every `AWSError` (a broader change to error output than this feature needs).
