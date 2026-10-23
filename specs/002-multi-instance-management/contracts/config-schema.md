# Contract: config.yaml (schema v2) and v1 Migration

Path: `~/.clouddesktop/config.yaml`. The file mode is 0600 and the directory mode is 0700, both unchanged.

## v2 schema

```yaml
aws_profile: test-terraform
region: us-east-1
developer_name: nico
ssh_public_key: ssh-ed25519 AAAA... nico@laptop
ssh_key_path: /Users/nico/.ssh/id_ed25519
instance_type: m7i.xlarge        # default machine size for NEW environments
default_environment: main        # optional; omitted when unset
```

| Field | Required | Notes |
|-------|----------|-------|
| `aws_profile`, `region`, `developer_name`, `ssh_public_key`, `ssh_key_path` | yes | Validation unchanged |
| `instance_type` | yes | Validation unchanged; meaning narrowed to "default for new environments" |
| `default_environment` | no | Empty, or a valid environment name (FR-002 rules) |

The file contains no per-environment data. Which environments exist comes from AWS tags ([external-state.md](./external-state.md)).

## v1 detection

A file is v1 when its top-level `instance_id` is non-empty.

```yaml
# v1 example
aws_profile: test-terraform
region: us-east-1
instance_type: m7i.xlarge
ssh_public_key: ssh-ed25519 AAAA...
ssh_key_path: /Users/nico/.ssh/id_ed25519
developer_name: nico
instance_id: i-0abc123
root_volume_size_gb: 200
```

## Migration contract

Migration runs inside `config.Load`, before validation, while holding the clouddesktop lock.

1. Write the original bytes unchanged to `~/.clouddesktop/config.yaml.v1-backup` with mode 0600. If a backup already exists, keep it and do not overwrite it.
2. Build the v2 config: copy the shared fields, set `default_environment: main`, and drop `instance_id` and `root_volume_size_gb`.
3. Write it to a temp file in the same directory with mode 0600, fsync it, and rename it over `config.yaml`.
4. Print once: `Migrated config to support multiple environments. Your existing instance is now environment "main". Backup: ~/.clouddesktop/config.yaml.v1-backup`.

**Guarantees**:
- If any step fails, `config.yaml` is still the original v1 file (FR-014).
- Migration makes no AWS calls and never touches the instance (FR-013).
- Running it again is a no-op, because v2 has no `instance_id`.
- Tagging the legacy instance happens later, on a best-effort basis (research R3), and migration doesn't depend on it.

**Not supported**: running an older clouddesktop binary against a v2 file. Restore from the backup to downgrade.
