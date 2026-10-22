# Contract: CLI Commands

This contract covers the user-facing command surface after this feature. `[name]` is an optional environment name, resolved by the rules in [research.md R5](../research.md#r5-default-environment-rules). Exit status is 0 on success and non-zero on any error (Cobra default).

## Target line (FR-023)

Every command that acts on one environment prints this line before it takes any action:

```text
Environment: <name> (<instance-id>)
```

Every confirmation prompt also includes `<name>`. `create` prints `Environment: <name> (creating)` before launching, because the instance ID doesn't exist yet, and prints the full line once it does.

## Commands

| Command | Arguments / flags | Behavior | Changes from today |
|---------|-------------------|----------|--------------------|
| `init` | none | Interactive setup, as today. After the profile and developer name are chosen, it lists that developer's environments and, if any exist, tells the developer to pick the SSH key they were created with. One environment becomes the default. With more than one, it prompts for the default. Refuses to run when a config exists and the developer has any environment (FR-021). | Adds discovery and the default prompt. The refusal condition now uses the AWS listing instead of `instance_id`. |
| `create <name>` | `--instance-type <type>` (default: config `instance_type`), `--storage-size <GB>` (default 100) | Validates the name, uniqueness and SSH alias collision, then prints `Environment: <name> (creating)`. Ensures the bucket exists, launches the instance with tags, waits until it is running, regenerates the SSH block, and becomes the default if it is the developer's first environment. Prints the summary and "run `clouddesktop up <name>` after bootstrap to mount the bucket". | New |
| `list`, `ls` | none | Table of NAME, STATE, TYPE, DISK and DEFAULT (`*`), followed by the bucket line. Never starts or changes an environment. | New |
| `use <name>` | none | Sets the default. Fails if `<name>` doesn't exist. | New |
| `up [name]` | none | Starts the target if stopped, waits until running, regenerates the SSH block, ensures the bucket exists, and ensures the mount on this instance. It prints `Setting up S3 mount on <name>...` first, and `Waiting for SSM to reach the instance...` if it has to wait. With no name and zero environments, it provisions `main` (same as `create main`). | Name argument; mount ensured per instance |
| `down [name]` | `--all` (cannot be combined with `name`) | Stops the target and waits. `--all` stops every running or pending environment, continues past failures, prints one result line per environment, and exits non-zero if any failed. With nothing running or pending, prints `No running environments.` and exits 0. | Name argument; `--all` |
| `status [name]` | none | As today, for the target. Also shows the environment name and whether it is the default. | Name argument |
| `ssh [name]` | none | Resolves the target, then execs `ssh clouddesktop-<name>`. | Name argument; now calls AWS to resolve the target |
| `resize-instance [name]` | none (interactive select, as today) | As today, for the target. Compares against the instance's actual type and doesn't write config. | Name argument; stops writing `instance_type` |
| `resize-storage [name]` | none (interactive, as today) | As today, for the target. Doesn't write config. | Name argument; stops writing `root_volume_size_gb` |
| `destroy [name]` | `--confirm` (required) | The name is required when the developer has more than one environment, and the default is never used (FR-024). Prints the FR-011 warning, terminates the target, regenerates the SSH block, clears the default if needed, and deletes the key pair only when it was the last environment. Never deletes the bucket. Keeps `config.yaml`. Without `--confirm`, prints the target line and the full warning, changes nothing, and exits non-zero. | Narrowed scope; config kept |
| `s3-reset` | `--confirm` (required) | Lists every environment that uses the bucket, then deletes the bucket and its parameter, as today. Without `--confirm`, prints the same list, changes nothing, and exits non-zero. | Lists affected environments |

## Naming convention

Name an environment after its function, in one word: `build`, `service`, `ci`. The `create` help text and the README show this convention. It is not enforced; hyphenated names are valid under FR-002.

## Error messages

Each error is returned as an error carrying remediation text, so it prints as one problem line plus one next-step line.

| Condition | Message (first line) | Remediation line |
|-----------|----------------------|------------------|
| Unknown name | `environment "<name>" not found` | `Your environments: main, build. Create one with 'clouddesktop create <name>'.` |
| No name, more than one environment, no default | `you have more than one environment and no default` | `Pass a name, or set a default with 'clouddesktop use <name>'. Your environments: ...` |
| Default points at a missing environment | `default environment "<name>" no longer exists` | `Set a new default with 'clouddesktop use <name>'. Your environments: ...` |
| Destroy without a name, more than one environment | `destroy requires an environment name when you have more than one` | `Run 'clouddesktop destroy <name> --confirm'. Your environments: ...` |
| No environments (any command but `up`, `create`, `list`, `init`, and `status` without a name, which prints this as information and exits 0) | `you have no environments` | `Create one with 'clouddesktop create <name>' or 'clouddesktop up'.` |
| Name taken | `environment "<name>" already exists` | `Choose another name.` |
| Invalid name | `invalid environment name "<name>"` | `Use 1-20 lowercase letters, digits or hyphens, starting and ending with a letter or digit.` |
| SSH alias collision | `~/.ssh/config already defines host "clouddesktop-<name>" outside the clouddesktop block` | `Choose another name or remove that entry.` |
| Duplicate tag | `two instances are tagged as environment "<name>": <id1>, <id2>` | `Terminate the extra instance in the AWS console, then retry.` |
| `down` with both a name and `--all` | `--all cannot be combined with an environment name` | n/a |
| `destroy` without `--confirm` | `re-run with --confirm to destroy environment "<name>"` | n/a (the warning printed above it says what would happen) |
| `s3-reset` without `--confirm` | `re-run with --confirm to delete bucket <bucket>` | n/a (the list printed above it shows the affected environments) |
| Account quota reached (`VcpuLimitExceeded`, `InstanceLimitExceeded`) | `the AWS account's on-demand instance limit has been reached` | `Stop environments you aren't using, or ask the platform team to raise the limit.` |
| No AWS capacity (`InsufficientInstanceCapacity`) | `AWS has no spare capacity for <type> right now` | `Try again in a few minutes, or choose another instance type.` |

## Output examples

`clouddesktop list`:

```text
NAME    STATE    TYPE         DISK     DEFAULT
build   stopped  m7i.4xlarge  300 GB
main    running  m7i.xlarge   100 GB   *

Personal bucket: clouddesktop-nico (available to all environments; kept when environments are destroyed)
```

`clouddesktop list` with no environments:

```text
You have no environments. Create one with 'clouddesktop create <name>'.

Personal bucket: clouddesktop-nico (exists; delete with 'clouddesktop s3-reset --confirm')
```

`clouddesktop destroy build --confirm`:

```text
Environment: build (i-0def...)
WARNING: This permanently destroys environment "build".
  All data on its disk (repos, Docker images, caches) will be DELETED.
  Your other environments are not affected: main
  Your personal bucket clouddesktop-nico is kept. Delete it with 'clouddesktop s3-reset --confirm'.
```

`clouddesktop s3-reset --confirm`:

```text
Deleting bucket clouddesktop-nico and all its contents.
Environments that use it: main (running), build (stopped)
Their mount will be repaired on their next 'clouddesktop up'.
```
