# Quickstart: Validating Multiple Cloud Desktops

Use this guide to check that the feature works end to end. Command behavior is defined in [contracts/cli.md](./contracts/cli.md), and the state formats are in [contracts/external-state.md](./contracts/external-state.md).

## Prerequisites

- A test AWS profile with access to the clouddesktop account (`us-east-1`), with an active session.
- `session-manager-plugin` and the `aws` CLI installed.
- A build from this branch: `make build`. Then run `export CLI=./bin/clouddesktop`; the commands below use `$CLI`.
- A disposable developer name, for example `qa-multienv`. Never test with your real developer name.
- Each scenario costs EC2 time. Run scenario 11 when you finish.

Each section notes the phase in tasks.md after which it can run. `list` doesn't exist until US2, so earlier sections use `status` and the config file instead.

## 1. Unit tests

```bash
make test
go test ./internal/environment/... ./internal/config/... ./internal/aws/...
make lint
```

**Expected**: all tests pass. The resolver table covers every row in research R5. Migration tests cover the backup, the atomic replace, the failure cases and idempotence.

## 2. Migration of an existing user (US4, FR-013, FR-014)

Runs after: Phase 3 (US4).

Setup: use the previous release binary to `init` and `up` with `qa-multienv`, and leave the instance running. Record the instance ID, `~/.ssh/config` and `~/.clouddesktop/config.yaml`.

```bash
$CLI status
cat ~/.clouddesktop/config.yaml ~/.clouddesktop/config.yaml.v1-backup
ssh clouddesktop hostname
aws ec2 describe-tags --filters Name=resource-id,Values=<instance-id> --profile <p>
```

**Expected**:
- `status` prints `Environment: main (<same instance ID>)`, then the same state and type as before the upgrade.
- The config has `default_environment: main` and no `instance_id`. The backup is identical to the original file.
- `ssh clouddesktop` reaches the same machine.
- The instance gained `CloudDesktopEnv=main`. If tagging was denied, `status` still resolves the instance as `main`.
- The instance state, type and launch time are unchanged.

## 3. Create an additional environment (US1)

Runs after: Phase 4 (US1).

```bash
$CLI create build --instance-type m7i.large
$CLI create build                         # duplicate
$CLI create Build_1                       # invalid
echo "Host clouddesktop-taken" >> ~/.ssh/config && $CLI create taken   # collision
grep default_environment ~/.clouddesktop/config.yaml
```

**Expected**:
- `create build` prints `Environment: build (creating)` before launching, then the full target line and the summary.
- `main` is unchanged, and the config still shows `default_environment: main`.
- The duplicate, invalid name and collision each fail with the error from the contract and create no AWS resources. Check this with `aws ec2 describe-instances --filters Name=tag:Developer,Values=qa-multienv`.
- Afterwards, remove the `clouddesktop-taken` line by hand.

## 4. List (US2, FR-007, SC-004)

Runs after: Phase 5 (US2).

```bash
$CLI down build && time $CLI list
```

**Expected**: both environments appear with the correct state, type and disk size. `main` is marked `*`. The bucket line is shown. The command takes under 10 s, and no state changes.

## 5. Targeting and the default (US3, FR-006, FR-022 to FR-024)

Runs after: Phase 6 (US3).

```bash
$CLI status            # acts on the default (main) and prints "Environment: main (...)"
$CLI use build && $CLI status          # acts on build
$CLI up main && $CLI down build        # by name; the other environment is unaffected
$CLI status nope                       # unknown name
$CLI destroy --confirm                 # refused: name required with more than one environment
```

**Expected**: every command prints the target line first. The unknown name and the destroy without a name fail with the contract messages, and nothing changes.

## 6. Bucket mount per environment (R8, F5 fix)

Runs after: Phase 6.

```bash
$CLI up build          # after bootstrap has completed on build
ssh clouddesktop-build 'ls ~/s3 && mount | grep /home/ubuntu/s3'
ssh clouddesktop-main 'touch ~/s3/from-main' && ssh clouddesktop-build 'ls ~/s3/from-main'
```

**Expected**: `up build` starts the stopped instance and finishes without a mount warning, because the mount command waits for SSM to reconnect (research R8). The bucket is mounted on both environments, and files written on one are visible on the other. Running `up build` before bootstrap finishes prints a warning containing "bootstrap in progress" and doesn't fail.

## 7. Destroy one environment (US3 scenario 2, FR-010 to FR-012, SC-002, SC-006)

Runs after: Phase 6.

```bash
$CLI destroy build                     # preview: prints the warning, changes nothing, exits non-zero
$CLI destroy build --confirm
$CLI s3-reset                          # preview: lists main as using the bucket, changes nothing
$CLI list && ssh clouddesktop-main 'ls ~/s3/from-main'
grep -c "Host clouddesktop-build" ~/.ssh/config
```

**Expected**:
- Both previews print what would happen, change nothing and exit non-zero.
- The warning names `build`, lists `main` as unaffected and says the bucket is kept.
- `main` is still running with its data and alias, and the bucket file still exists.
- The `build` alias is gone (count 0). The key pair `clouddesktop-qa-multienv` still exists.
- `build` was the default (set in section 5), so the default is cleared and `list` shows no `*`.

## 8. Recovery on a new machine (US5, FR-015)

Runs after: Phase 7 (US5).

```bash
$CLI create extra
mv ~/.clouddesktop ~/.clouddesktop.bak && $CLI init   # same profile and developer name
$CLI list
```

**Expected**: `init` finds `main` and `extra` right after the profile prompt and reminds you to pick the same SSH key. It then asks which environment is the default and creates nothing. `list` shows both.

## 9. Stop all and concurrency (US6, FR-016, FR-019)

Runs after: Phase 8 (US6).

```bash
$CLI up main & $CLI up extra & wait
grep -c "^Host clouddesktop-" ~/.ssh/config     # expect both aliases present
$CLI down --all
$CLI down --all                                  # again, with nothing running
```

**Expected**:
- The concurrent `up`s both succeed, and the SSH block contains both environments.
- The first `down --all` prints one line per environment, and both end up stopped. The second prints `No running environments.` and exits 0.
- To check failure handling, deny `StopInstances` for one instance (or stop it from the console mid-run). The other environment still stops, and the command exits non-zero.

## 10. Idle auto-stop per environment (FR-018)

Runs after: Phase 8. This takes more than 4 hours, so run it overnight.

```bash
$CLI up main && $CLI up extra
ssh clouddesktop-main 'systemctl is-active clouddesktop-autostop.timer'
ssh clouddesktop-extra 'systemctl is-active clouddesktop-autostop.timer'
ssh clouddesktop-main 'tmux new -d -s keepalive "sleep 18000"'   # keeps main active for 5 hours
# Do not connect to extra for more than 4 hours, then:
$CLI list
```

**Expected**: the timer is active on both environments. After 4 hours (16 idle checks, 15 minutes apart), `extra` has stopped on its own, and `main` is still running because its tmux session counts as activity.

## 11. Cleanup

Runs last.

```bash
$CLI destroy main --confirm && $CLI destroy extra --confirm
$CLI list                          # no environments; bucket still listed
$CLI s3-reset --confirm            # explicit bucket delete
aws ec2 describe-key-pairs --key-names clouddesktop-qa-multienv --profile <p>   # expect NotFound
```

**Expected**: no instances, key pair or bucket remain for `qa-multienv`. `~/.clouddesktop/config.yaml` still exists.
