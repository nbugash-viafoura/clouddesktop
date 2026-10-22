# Data Model: Multiple Cloud Desktops per Developer

Phase 1 output for [plan.md](./plan.md). Exact formats are in [contracts/](./contracts/), and the reasoning is in [research.md](./research.md).

## Overview

```text
DeveloperSettings (local file) ---- default_environment ----> Environment.name
        | developer_name
        v
Environment (EC2 instance + tags) 1..N per developer
        | all mount
        v
PersonalBucket (1 per developer)        ManagedSSHBlock = f(all Environments)
```

## DeveloperSettings

Stored in `~/.clouddesktop/config.yaml`, schema v2, mode 0600. See [contracts/config-schema.md](./contracts/config-schema.md).

| Field | Type | Rules |
|-------|------|-------|
| `aws_profile` | string | Required (unchanged) |
| `region` | string | Required, must be an enabled region (unchanged) |
| `developer_name` | string | Required, `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$` (unchanged) |
| `ssh_public_key` | string | Required, starts with `ssh-` (unchanged) |
| `ssh_key_path` | string | Required (unchanged) |
| `instance_type` | string | Required, from `ValidInstanceTypes`. **Meaning changes** to "default machine size for new environments". It no longer tracks any environment's actual size. |
| `default_environment` | string | Optional. Empty, or a valid environment name. Existence is checked at resolution time, not at load time. |

Legacy fields, read only during migration and never written: `instance_id`, `root_volume_size_gb`.

Changing `developer_name` or `ssh_public_key` is refused by `init` while any environment exists (FR-021).

## Environment

Derived from an EC2 instance on every command. Nothing about it is stored locally.

| Field | Source | Notes |
|-------|--------|-------|
| `Name` | Tag `CloudDesktopEnv`, or `main` when the tag is absent (legacy) | Unique per developer |
| `InstanceID` | Instance | |
| `State` | Instance state | pending, running, stopping or stopped. Other states are excluded from listing. |
| `InstanceType` | Instance | Source of truth for the machine size (replaces `cfg.InstanceType`) |
| `RootVolumeID` | Block device mapping that matches `RootDeviceName` | |
| `RootVolumeSizeGB` | DescribeVolumes | Filled only by commands that display it (`list`, `status`) |
| `PrivateIP`, `LaunchTime` | Instance | For display |
| `Legacy` | True when `CloudDesktopEnv` is absent | Triggers best-effort tagging (research R3) |

**Validation (on create)**:
- The name matches the developer-name pattern and is at most 20 characters (FR-002).
- The name is not already used by one of the developer's environments.
- Alias `clouddesktop-<name>` is not on a `Host` line outside the managed SSH block.
- The machine size is in `ValidInstanceTypes`, and the disk size is in `ValidStorageSizes`.

**Derived values**:
- SSH aliases: `clouddesktop-<name>`, plus `clouddesktop` when the name is `main`.
- Name tag: `clouddesktop-<developer>-<name>` for new instances; legacy instances keep `clouddesktop-<developer>`.

**Integrity rule**: two listed instances with the same `Name` cause a duplicate-name error that names both instance IDs. The tool never picks one.

### Lifecycle

```text
          create / up (no envs)
(none) ------------------------> pending --> running <--> stopping --> stopped
                                               ^                         |
                                               +----------- up ----------+
any listed state --destroy--> shutting-down --> terminated   (both excluded from listing)
```

Idle auto-stop moves an environment from running to stopped on its own, independently of other environments (FR-018).

## Default environment pointer

`DeveloperSettings.default_environment`. Resolution rules are in research R5.

| Event | Effect |
|-------|--------|
| v1 config migrated | Set to `main` |
| Environment created while the developer has zero environments | Set to the new environment |
| Environment created while others exist | Unchanged |
| `use <name>` (name exists) | Set to `<name>` |
| Default environment destroyed | Cleared, never reassigned automatically |
| `init` discovers more than one environment | Developer is prompted to choose |
| `init` discovers exactly one environment | Set to it |

## PersonalBucket

| Attribute | Value |
|-----------|-------|
| Name | `clouddesktop-<developer>` (unchanged) |
| Pointer | SSM parameter `/clouddesktop/developer/<developer>/s3_bucket_name` (unchanged) |
| Cardinality | One per developer, shared by all of that developer's environments |

```text
absent --(provision, or up on a running env)--> exists --(s3-reset --confirm)--> absent
```

`destroy` never changes the bucket's state (FR-012).

## MountState (per instance, on the instance)

```text
bootstrap-pending --(bootstrap completes)--> unmounted --(up: ensure mount)--> mounted-healthy
mounted-healthy --(bucket deleted by s3-reset)--> mounted-stale --(up: ensure mount)--> mounted-healthy
```

The ensure-mount step does nothing for `mounted-healthy` and reports `bootstrap-pending` without failing `up` (research R8).

## ManagedSSHBlock

This is a pure function of (environments, `ssh_key_path`, proxy script path). It holds one `Host` entry per listed environment, in name order for stable diffs. When there are no environments, the block and the proxy script are removed. Format: [contracts/external-state.md](./contracts/external-state.md).

## Config migration (v1 to v2)

```text
v1 file --read--> write backup (0600) --> write v2 to temp file --> rename over config.yaml --> v2
                     | fail                       | fail
                     v                            v
               v1 untouched, error          v1 untouched, error
```

A v1 file is detected by a non-empty top-level `instance_id`. After a successful migration, a v2 file has no `instance_id`, so migration is idempotent.
