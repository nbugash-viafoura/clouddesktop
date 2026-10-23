# Contract: External State (AWS resources, SSH config, lock file)

This contract covers state that other tools and people read: cost reports, the AWS console, SSH clients and remote editors.

## EC2 instance tags

| Key | Value | New instances | Legacy instance |
|-----|-------|---------------|-----------------|
| `Name` | `clouddesktop-<developer>-<env>` | Set at launch | Keeps `clouddesktop-<developer>` |
| `Developer` | `<developer>` | Set at launch (unchanged) | Present |
| `Project` | `clouddesktop` | Set at launch (unchanged) | Present |
| `ManagedBy` | `clouddesktop-cli` | Set at launch (unchanged) | Present |
| `CloudDesktopEnv` | `<env>` | Set at launch | Added best-effort as `main`. When absent, it is read as `main`. |

## Root volume tags

| Key | Value |
|-----|-------|
| `Name` | `clouddesktop-<developer>-<env>-root` (legacy volumes keep `clouddesktop-<developer>-root`) |
| `Developer` | `<developer>` |
| `CloudDesktopEnv` | `<env>` (new volumes only) |

For FR-020 to show up in AWS cost reports, an account administrator must activate `Developer` and `CloudDesktopEnv` as cost allocation tags. That is a one-time step outside the CLI.

## Listing query

DescribeInstances is called with these filters:
- `tag:Developer=<developer>`
- `tag:Project=clouddesktop`
- `instance-state-name in (pending, running, stopping, stopped)`

All result pages are read.

## Other AWS resources

| Resource | Name | Lifecycle |
|----------|------|-----------|
| EC2 key pair | `clouddesktop-<developer>` | Imported before each launch, and "already exists" counts as success. Deleted only when the last environment is destroyed. |
| S3 bucket | `clouddesktop-<developer>` | Created when missing (provision, or `up`). Deleted only by `s3-reset --confirm`. |
| SSM parameter | `/clouddesktop/developer/<developer>/s3_bucket_name` | Same lifecycle as the bucket |

## Managed SSH block (`~/.ssh/config`)

The markers and the proxy script (`~/.ssh/clouddesktop-ssm-proxy.sh`) are unchanged. The block lists one `Host` per environment, sorted by name, with a blank line between entries:

```text
# BEGIN clouddesktop managed block
Host clouddesktop-build
  HostName i-0def456
  User ubuntu
  IdentityFile /Users/nico/.ssh/id_ed25519
  ForwardAgent yes
  ProxyCommand /Users/nico/.ssh/clouddesktop-ssm-proxy.sh %h %p
  ServerAliveInterval 60
  ServerAliveCountMax 3
  StrictHostKeyChecking accept-new

Host clouddesktop-main clouddesktop
  HostName i-0abc123
  User ubuntu
  IdentityFile /Users/nico/.ssh/id_ed25519
  ForwardAgent yes
  ProxyCommand /Users/nico/.ssh/clouddesktop-ssm-proxy.sh %h %p
  ServerAliveInterval 60
  ServerAliveCountMax 3
  StrictHostKeyChecking accept-new
# END clouddesktop managed block
```

**Rules**:
- The block content is regenerated from the full environment list on every write, and content outside the markers is never modified.
- The existing backup to `~/.ssh/config.clouddesktop-backup` still happens before each write.
- Writes go to a temp file in the target's directory and are then renamed. If `~/.ssh/config` is a symlink, the resolved target is written.
- With zero environments, the block and the proxy script are removed.
- Collision check, before `create`: any `Host` line outside the block that contains the token `clouddesktop-<name>` blocks the name. `Include`d files are not inspected.

## Lock file

`~/.clouddesktop/.lock` holds an exclusive advisory `flock`. It is held only around config read-modify-write and SSH block regeneration, never during AWS waits. The file is created when missing, with mode 0600.
