# Implementation Plan: Multiple Cloud Desktops per Developer

**Branch**: `feature/multi-ec2-instance` | **Date**: 2026-10-21 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/002-multi-instance-management/spec.md`

## Summary

Let a developer own several named cloud desktops and operate each one by name, with a switchable default for everyday commands. The design makes AWS instance tags the single source of truth for which environments exist: every instance carries `CloudDesktopEnv=<name>`, and the local config keeps only developer settings plus a `default_environment` pointer. Each command lists the developer's environments with one tag-filtered call, resolves its target through a single pure function, and prints the target before acting.

Existing users migrate on first run. The config file is rewritten atomically with a backup, the legacy instance becomes `main`, and the `clouddesktop` SSH alias keeps pointing to it. Destroy is narrowed to the target environment: it never deletes the personal bucket, and it deletes the shared key pair only with the last environment. The S3 mount is now ensured per instance. That fixes a bug in today's code where freshly provisioned instances never get the mount. See [research.md](./research.md) for every decision and the alternatives considered.

## Technical Context

**Language/Version**: Go 1.24

**Primary Dependencies**: aws-sdk-go-v2 (ec2, ssm, s3, sts, cloudwatch), spf13/cobra, gopkg.in/yaml.v3, charmbracelet/huh. No new dependencies; the file lock uses `syscall.Flock`, which is available on both build targets (darwin, linux).

**Storage**:
- `~/.clouddesktop/config.yaml` (0600): developer settings and `default_environment` (schema v2).
- EC2 instance and volume tags: the environment registry.
- SSM Parameter Store: the personal bucket name (unchanged).
- `~/.ssh/config`: the managed block, regenerated from the environment list.

**Testing**: `go test` with table-driven tests. AWS API-level mocks are injected through the existing `newEC2ClientWithAPI` / `newSSMClientWithAPI` constructors. File-based tests use `t.TempDir()`. Manual end-to-end steps are in [quickstart.md](./quickstart.md).

**Target Platform**: CLI on macOS (arm64, amd64) and Linux (amd64). Remote instances run Ubuntu 24.04 on EC2 in `us-east-1`.

**Project Type**: CLI

**Performance Goals**: `list` returns within 10 s for 10 environments (SC-004). It uses a constant number of AWS calls regardless of environment count: one paginated DescribeInstances, one DescribeVolumes and one bucket check.

**Constraints**:
- No inbound network access and no changes to `terraform/shared`.
- Existing v1 configs and instances must keep working without re-provisioning (FR-013).
- SSH config entries outside the managed block must never be modified (FR-009).
- Destroy never deletes the personal bucket (FR-012).

**Scale/Scope**: Typically 1-5 environments per developer, with tens of developers sharing one AWS account and its on-demand vCPU quota. The change touches about 12 existing files and adds 2 packages (`environment`, `fsutil`) and 3 commands.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unfilled template, so there are no ratified principles to gate on. Until a constitution is ratified (`/speckit-constitution`), the standards in the project and user `CLAUDE.md` serve as the gates:

| Gate | Pre-design | Post-design |
|------|------------|-------------|
| AWS access stays behind injectable API interfaces so logic is unit-testable | Pass | Pass. New EC2 calls (CreateTags, paginated DescribeInstances) go through `ec2api`. The resolver, migration and SSH rendering are pure or file-based. |
| Config remains 0600 and validated on load and save | Pass | Pass. The v2 schema adds `default_environment` with name validation. Migration backups are written 0600. |
| User-facing errors state the next step | Pass | Pass. `main.go` prints only `Error()`, so the new errors (unknown environment, name required, no default, duplicate name, capacity) put the next step in the message itself (T008, research R13). |
| No dead code or TODOs | Pass | Pass. `FindInstanceByDeveloper` is replaced, not kept beside `ListEnvironments`. |
| Security: no new IAM grants, no inbound ports, least change to shared infrastructure | Pass | Pass. Pre-existing cross-developer S3 access is recorded in research R12 as out of scope. |
| Business logic has unit tests | Pass | Pass. See research R11. |

No violations, so Complexity Tracking is empty.

## Project Structure

### Documentation (this feature)

```text
specs/002-multi-instance-management/
├── spec.md
├── plan.md                     # This file
├── research.md                 # Phase 0: decisions R1-R13
├── data-model.md               # Phase 1: entities, rules, state transitions
├── quickstart.md               # Phase 1: end-to-end validation guide
├── contracts/
│   ├── cli.md                  # Command surface, arguments, errors, output
│   ├── config-schema.md        # config.yaml v2 and v1 migration contract
│   └── external-state.md       # AWS tags/names, managed SSH block, lock file
├── checklists/requirements.md
└── tasks.md                    # Phase 2 (/speckit-tasks), not created here
```

### Source Code (repository root)

```text
internal/
├── environment/                # NEW: pure domain logic, no AWS or file I/O
│   ├── environment.go          # Environment type, name validation, SSH aliases
│   ├── resolve.go              # Target resolution rules (research R5)
│   └── *_test.go
├── fsutil/                     # NEW: WriteFileAtomic (temp + fsync + rename, symlink-safe), shared by config and sshconfig
│   └── atomic.go, atomic_test.go
├── config/
│   ├── config.go               # v2 schema: default_environment; instance_type = default for new envs
│   ├── migrate.go              # NEW: v1 -> v2 with backup and atomic replace
│   ├── lock.go                 # NEW: advisory flock on ~/.clouddesktop/.lock
│   └── *_test.go
├── aws/
│   ├── ec2.go                  # ListEnvironments (paginated), CloudDesktopEnv tags, TagEnvironment, root volume size param, capacity errors
│   ├── types.go                # ErrorTypeCapacity (research R13)
│   ├── provisioner.go          # Env-aware Provision (recovery by developer+env) and Destroy (no bucket deletion)
│   ├── sshconfig.go            # Multi-host managed block, outside-block collision check, atomic symlink-safe write
│   ├── ssm.go                  # Idempotent mount script with health check and bootstrap guard; SendCommand retry while SSM reconnects
│   └── *_test.go
├── cli/
│   ├── target.go               # NEW: load config -> validate session -> list envs -> resolve -> print target
│   ├── create.go               # NEW
│   ├── list.go                 # NEW
│   ├── use.go                  # NEW
│   ├── list_test.go, down_test.go   # NEW: unit tests for renderEnvironmentTable and stopAll
│   ├── root.go                 # Register new commands
│   ├── up.go, down.go, status.go, ssh.go, resize.go, resize_storage.go,
│   ├── destroy.go, s3_reset.go, init.go   # Accept [name]; down --all; init discovery and default prompt
└── testing/mock.go             # Extend mocks as needed
README.md, CLAUDE.md            # Command reference, destroy behavior change, bootstrap script name
```

**Structure Decision**: Keep the existing single-module CLI layout. Add one small `internal/environment` package so the resolution rules (the core of User Story 3) are pure, table-testable and free of AWS or file dependencies. `internal/aws` imports it for the `Environment` type, and `internal/cli` stays thin glue that matches today's untested command files.

## Complexity Tracking

No constitution violations to justify.
