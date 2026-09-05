# Cloud Desktop CLI

A self-service CLI tool for provisioning and managing personal cloud development environments on AWS EC2. Each developer gets their own instance with Docker, Java 21, Node.js, and all the tooling needed to run Viafoura services remotely.

## Table of Contents
<details>
  <summary>See table of contents</summary>

- [Why](#why)
- [How It Works](#how-it-works)
  - [Auto-Stop](#auto-stop)
  - [Cost](#cost)
- [Prerequisites](#prerequisites)
  - [1. AWS CLI v2](#1-aws-cli-v2)
  - [2. SSM Session Manager Plugin](#2-ssm-session-manager-plugin)
  - [3. SSH Key Pair](#3-ssh-key-pair)
- [Installation](#installation)
  - [Download (recommended)](#download-recommended)
  - [Build from source](#build-from-source)
- [Usage](#usage)
  - [First-Time Setup](#first-time-setup)
  - [Provision and Start](#provision-and-start)
  - [Connect](#connect)
  - [Check Status](#check-status)
  - [Stop](#stop)
  - [Resize Instance Type](#resize-instance-type)
  - [Resize Storage](#resize-storage)
  - [S3 Mount](#s3-mount)
  - [Reset S3 Bucket](#reset-s3-bucket)
  - [Destroy](#destroy)
- [Commands Reference](#commands-reference)
- [Daily Workflow](#daily-workflow)
- [What's Installed on the Instance](#whats-installed-on-the-instance)
- [Architecture](#architecture)
  - [Instance State](#instance-state)
- [Repository Structure](#repository-structure)
- [Deployment (Admin Only)](#deployment-admin-only)
  - [Shared Infrastructure (Tier 1)](#shared-infrastructure-tier-1)
  - [Verifying the Deployment](#verifying-the-deployment)
  - [Tearing Down Shared Infrastructure](#tearing-down-shared-infrastructure)
- [Troubleshooting](#troubleshooting)
  - [SSH Agent Forwarding Issues](#ssh-agent-forwarding-issues)
  
</details>

## Why

Local developer machines allocate 8 CPU + 16 GB RAM to Colima(if you're using MacOS), leaving the laptop in survival mode for the IDE and browser. `clouddesktop` moves that compute to an EC2 instance where Docker runs natively (no VM layer, no hypervisor overhead). The local machine only runs the IDE frontend — all builds, containers, and tests execute on the remote instance.

## How It Works

- **Provision**: `clouddesktop up` creates an EC2 instance via the AWS SDK on first run (~2 minutes)
- **Start/Stop**: subsequent `clouddesktop up` and `clouddesktop down` start and stop the instance (~30 seconds)
- **Persist**: stopping an instance preserves the EBS volume — repos, Docker image cache, Gradle/npm caches, shell history all survive across sessions
- **Connect**: SSH tunneled through AWS SSM Session Manager (zero open inbound ports). JetBrains Gateway and VS Code Remote SSH work transparently via the SSH config entry `clouddesktop` writes automatically
- **S3 Mount**: each developer gets a personal S3 bucket mounted at `/home/ubuntu/s3` via [Mountpoint for Amazon S3](https://github.com/awslabs/mountpoint-s3). Files published to the bucket are visible as local files for ETL workloads. The mount persists across instance stop/start cycles.
- **Destroy**: `clouddesktop destroy --confirm` terminates the instance and deletes all data (including the S3 bucket). Requires explicit confirmation.

### Auto-Stop

Each instance runs a systemd timer (`clouddesktop-autostop`) that checks for activity every 15 minutes and **shuts the instance down after roughly 4 hours of inactivity**. This protects against forgotten running instances. An instance is considered active if any of the following is true:

- An established SSH connection on port 22 (e.g. an open terminal, JetBrains Gateway, or VS Code Remote SSH session)
- A login session in `who`
- A running `tmux` or `screen` session (so detached long-running work is not interrupted)

After an auto-stop, your data is preserved on the EBS volume — run `clouddesktop up` to start the instance again (~30 seconds). The idle check does not run until the first-boot bootstrap has completed.

### Cost

Compute cost is incurred only while the instance is running (and an idle instance auto-stops after ~4 hours — see [Auto-Stop](#auto-stop)). Stopped instances only incur EBS storage cost.

| Instance type | Running (10h/day, 22 days/month) | Stopped (EBS only) | Monthly estimate |
|---|---|---|---|
| `m7i.xlarge` (4 vCPU, 16 GB) | ~$44 | ~$8 | ~$52 |
| `m7i.2xlarge` (8 vCPU, 32 GB) | ~$89 | ~$8 | ~$97 |
| `r7i.2xlarge` (8 vCPU, 64 GB) | ~$116 | ~$8 | ~$124 |
| `m7i.4xlarge` (16 vCPU, 64 GB) | ~$177 | ~$8 | ~$185 |
| `r7i.4xlarge` (16 vCPU, 128 GB) | ~$233 | ~$8 | ~$241 |
| `m7i.8xlarge` (32 vCPU, 128 GB) | ~$355 | ~$8 | ~$363 |
| `m7i.12xlarge` (48 vCPU, 192 GB) | ~$532 | ~$8 | ~$540 |

If you need more than 64 GB of RAM but not the extra cores, prefer the `r7i` family — it gives 8 GB per vCPU instead of 4, so `r7i.4xlarge` reaches 128 GB at roughly two-thirds the cost of `m7i.8xlarge`.

## Prerequisites

Install the following on your local machine before using `clouddesktop`:

### 1. AWS CLI v2

macOS:

```bash
brew install awscli
```

Ubuntu

```bash
apt install awscli
```

Verify your AWS profile is configured. `clouddesktop init` defaults to the `test-terraform` profile, but you can select any configured profile during setup:

```bash
aws sts get-caller-identity --profile test-terraform   # substitute your own profile if you use a different one
```

If your session has expired, run `sts` to refresh MFA credentials first.

### 2. SSM Session Manager Plugin

Required for SSH access through SSM (no direct port 22):

macOS:

```bash
brew install --cask session-manager-plugin
```

Ubuntu:

```bash
curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb" -o "session-manager-plugin.deb"
sudo dpkg -i session-manager-plugin.deb
rm session-manager-plugin.deb
```

Verify the installation:

```bash
session-manager-plugin
```

### 3. SSH Key Pair

`clouddesktop` uses your existing SSH key pair -- the same one you use for GitHub. During `clouddesktop init`, you'll provide the path to your public key (e.g., `~/.ssh/id_ed25519.pub`). The public key is uploaded to AWS during provisioning; the private key stays on your laptop.

Make sure your key is loaded in your SSH agent before connecting:

```bash
ssh-add ~/.ssh/id_ed25519    # or whichever key you use for GitHub
ssh-add -l                   # verify it's loaded
```

SSH agent forwarding is enabled automatically, so `git clone` and `git push` on the instance authenticate using your local key without copying it to the remote machine.

## Installation

### Download (recommended)

Requires the [GitHub CLI](https://cli.github.com/) (`gh`) — the repo is private so downloads need authentication.

**macOS (Apple Silicon)**
```bash
gh release download --repo nbugash-viafoura/clouddesktop --pattern "clouddesktop-*-darwin-arm64.tar.gz"
tar -xzf clouddesktop-*-darwin-arm64.tar.gz
sudo mkdir -p /usr/local/bin && sudo mv clouddesktop /usr/local/bin/clouddesktop
```

**macOS (Intel)**
```bash
gh release download --repo nbugash-viafoura/clouddesktop --pattern "clouddesktop-*-darwin-amd64.tar.gz"
tar -xzf clouddesktop-*-darwin-amd64.tar.gz
sudo mkdir -p /usr/local/bin && sudo mv clouddesktop /usr/local/bin/clouddesktop
```

**Linux (amd64)**
```bash
gh release download --repo nbugash-viafoura/clouddesktop --pattern "clouddesktop-*-linux-amd64.tar.gz"
tar -xzf clouddesktop-*-linux-amd64.tar.gz
sudo mv clouddesktop /usr/local/bin/clouddesktop
```

Verify with `clouddesktop --version`.

### Build from source

Requires Go 1.23+:

```bash
git clone git@github.com:nbugash-viafoura/clouddesktop.git
cd clouddesktop
make build && make install
```

This installs the `clouddesktop` binary to `/usr/local/bin/clouddesktop`.

## Usage

### First-Time Setup

```bash
clouddesktop init
```

Prompts for:
- **Developer name** — lowercase, used in AWS resource naming (e.g., `john`)
- **AWS profile** — picker of your configured AWS profiles; defaults to `test-terraform`
- **Instance type** — picker of supported types; defaults to `m7i.xlarge` (4 vCPU, 16 GB)
- **SSH public key** — picker of the `.pub` keys found in `~/.ssh/`

Region is fixed to `us-east-1`.

Writes configuration to `~/.clouddesktop/config.yaml`.

### Provision and Start

```bash
clouddesktop up
```

On first run, this provisions an EC2 instance (~2 minutes for infrastructure, ~10 minutes for bootstrap). On subsequent runs, it starts the existing stopped instance (~30 seconds).

On first run, `clouddesktop up` also creates a personal S3 bucket (`clouddesktop-<your-name>`) and mounts it at `/home/ubuntu/s3`. For existing instances that predate this feature, running `clouddesktop up` will automatically set up the S3 mount.

After the instance is running, `clouddesktop up` automatically writes an SSH config entry to `~/.ssh/config`:

```
# BEGIN clouddesktop managed block
Host clouddesktop
  HostName <instance-id>
  User ubuntu
  IdentityFile ~/.ssh/<your-key>
  ForwardAgent yes
  ProxyCommand ~/.ssh/clouddesktop-ssm-proxy.sh %h %p
  ServerAliveInterval 60
  ServerAliveCountMax 3
  StrictHostKeyChecking accept-new
# END clouddesktop managed block
```

The `ProxyCommand` points at a generated wrapper script (`~/.ssh/clouddesktop-ssm-proxy.sh`) rather than an inline command. The wrapper resolves the absolute paths to `aws` and `session-manager-plugin`, then runs `aws ssm start-session` against your instance using the AWS profile and region from your config. The block is delimited by the `# BEGIN/END clouddesktop managed block` markers so the CLI can update it in place without disturbing the rest of `~/.ssh/config` (your existing config is backed up to `~/.ssh/config.clouddesktop-backup` on each write).

### Connect

```bash
clouddesktop ssh
```

Or connect directly:

```bash
ssh clouddesktop
```

Both JetBrains Gateway and VS Code Remote SSH pick up the `clouddesktop` host from `~/.ssh/config` automatically.

**JetBrains Gateway** (backend devs):
1. Install JetBrains Gateway locally
2. New Connection > SSH > select `clouddesktop`
3. First connection downloads the IDE backend to the instance (~3-5 min, one-time)

**VS Code Remote SSH** (frontend devs):
1. Install the "Remote - SSH" extension
2. `Cmd+Shift+P` > Remote-SSH: Connect to Host > `clouddesktop`

### Check Status

```bash
clouddesktop status
```

Shows instance state, type, IP address, uptime, and CloudWatch metrics (CPU, memory, disk).

### Stop

```bash
clouddesktop down
```

Stops the instance. All data is preserved on the EBS volume. No compute cost while stopped.

### Resize Instance Type

```bash
clouddesktop resize-instance
```

Presents an interactive picker of supported instance types. Stops the instance if running, applies the change, then restarts. Use this if CloudWatch metrics show you need more CPU or memory.

### Resize Storage

```bash
clouddesktop resize-storage
```

Grows the root EBS volume online — no instance stop required. Presents a picker of supported sizes larger than the current volume (100, 200, 300, 500, 1024, 1536, 2048 GB), then automatically extends the filesystem via SSM. Hard cap is 2 TB; EBS volumes cannot be shrunk.

### S3 Mount

Each developer gets a personal S3 bucket mounted at `/home/ubuntu/s3` using [Mountpoint for Amazon S3](https://github.com/awslabs/mountpoint-s3). This is set up automatically during `clouddesktop up`.

**Use cases:**
- Receive files published to the bucket by other systems or teammates for ETL processing
- Persist large datasets that don't need to live on the EBS volume
- Share output files by writing to `/home/ubuntu/s3`

**Behavior:**
- The bucket is created automatically on first `clouddesktop up`
- The mount survives `clouddesktop down` / `clouddesktop up` cycles (fstab auto-remounts on boot)
- Files uploaded to the S3 bucket externally (console, CLI, other services) appear immediately on the instance
- The bucket and all its contents are deleted on `clouddesktop destroy`
- Existing instances that predate this feature get the mount automatically on the next `clouddesktop up`

**Uploading files to a developer's bucket externally:**
```bash
aws s3 cp data.csv s3://clouddesktop-<developer-name>/data.csv --profile test-developers
```

**Limitations (Mountpoint for S3):**
- No in-place modification of existing files (write-once; delete and rewrite to update)
- No file locking, symlinks, or `chmod`/`chown`
- Reads and new file writes work normally

### Reset S3 Bucket

```bash
clouddesktop s3-reset --confirm
```

Deletes the developer's S3 bucket (and all its contents) and removes the backing SSM parameter. This is an AWS-side operation only — it does not unmount anything on the instance, so an existing `/home/ubuntu/s3` mount keeps pointing at a bucket that no longer exists until the next `clouddesktop up`, which creates a fresh bucket and re-runs mount setup. Use this if the bucket is in a bad state or you want to start over with empty S3 storage. The `--confirm` flag is required because it deletes all bucket contents. The EC2 instance and its EBS data are untouched.

### Destroy

```bash
clouddesktop destroy --confirm
```

Permanently terminates the instance and deletes all associated resources (EBS volume, key pair, S3 bucket and its contents). The `--confirm` flag is required. After destroying, run `clouddesktop init` to start fresh.

## Commands Reference

| Command | Description |
|---|---|
| `clouddesktop init` | One-time setup: configure AWS profile, instance type, SSH key |
| `clouddesktop up` | Start instance (or provision on first run) |
| `clouddesktop down` | Stop instance (data preserved) |
| `clouddesktop status` | Show instance state, IP, uptime, metrics |
| `clouddesktop ssh` | Open SSH session to instance |
| `clouddesktop resize-instance` | Change instance type interactively |
| `clouddesktop resize-storage` | Grow root EBS volume and extend filesystem online |
| `clouddesktop s3-reset --confirm` | Delete and recreate the S3 bucket and mount |
| `clouddesktop destroy --confirm` | Permanently delete instance and all data |

## Daily Workflow

```bash
sts                  # Refresh AWS MFA session (if expired)
clouddesktop up      # Start your instance
# Open JetBrains Gateway or VS Code Remote SSH to clouddesktop
# Work normally — all Docker, builds, tests run on the remote instance
clouddesktop down    # End of day: stop instance
```

## What's Installed on the Instance

The bootstrap script (`scripts/bootstrap-system.sh`) runs automatically on first provision and installs:

| Tool | Version | Notes |
|---|---|---|
| Docker Engine + Compose v2 | Latest stable | Native Linux Docker, no VM |
| Java (Corretto) | 21 | Via SDKMAN, matches `.sdkmanrc` |
| Node.js | LTS (v22) | Via fnm |
| Claude Code CLI | Latest | `npm install -g @anthropic-ai/claude-code` |
| AWS CLI v2 | Latest | For ECR and other AWS operations |
| ECR credential helper | Latest | `docker pull` from ECR works without manual login |
| zsh + oh-my-zsh | Latest | Default shell |
| tmux, git, make, jq | Latest | Standard tooling |
| Mountpoint for S3 | Latest | Mounts personal S3 bucket at `/home/ubuntu/s3` |
| CloudWatch Agent | Latest | Publishes CPU, memory, disk metrics |
| SSM Agent | Pre-installed | Enables SSH via SSM Session Manager |

ECR authentication is handled automatically via the instance's IAM role — no `aws ecr get-login-password` needed.

## Architecture

```
Developer Laptop                         AWS (us-east-1)
+-----------------+                      +---------------------------+
| IDE (Gateway /  |  SSH over SSM        | EC2 (m7i.xlarge)          |
| VS Code Remote) | -------------------> |   Docker Engine           |
|                 |  ProxyCommand        |   Java 21 + Gradle        |
| clouddesktop CLI|                      |   Node.js + npm/pnpm      |
+-----------------+                      |   Claude Code CLI         |
                                         |   100 GB gp3 EBS          |
                                         |   /home/ubuntu/s3 (mount) |
                                         +---------------------------+
                                                    |
                                                    | Mountpoint for S3
                                                    v
                                         +---------------------------+
                                         | S3 Bucket (per developer) |
                                         | clouddesktop-<name>       |
                                         +---------------------------+
                                         | IAM Instance Profile      |
                                         |   - SSM access            |
                                         |   - ECR read-only         |
                                         |   - CloudWatch metrics    |
                                         |   - S3 bucket access      |
                                         +---------------------------+
                                         | Security Group            |
                                         |   - Zero inbound rules    |
                                         |   - All outbound allowed  |
                                         +---------------------------+
```

### Instance State

Each developer's instance configuration is stored locally in `~/.clouddesktop/config.yaml`. This file tracks the instance ID, developer name, AWS profile, region, and instance type. It is created by `clouddesktop init` and updated automatically as you provision, resize, or destroy instances.

## Repository Structure

```
clouddesktop/
  cmd/clouddesktop/main.go       # CLI entry point
  internal/
    cli/                         # One file per command (init, up, down, status, ssh, resize, resize-storage, s3-reset, destroy)
    aws/                         # AWS SDK clients (EC2, SSM, S3, CloudWatch, STS, provisioner)
    config/                      # ~/.clouddesktop/config.yaml read/write
    testing/                     # Mock AWS clients used by unit tests
    version/                     # Build-time version info (injected via ldflags)
  terraform/
    shared/                      # One-time shared infra (IAM, SG, SSM params, state backend)
  scripts/
    bootstrap-system.sh          # System tooling (embedded, runs via user_data on first provision)
    embed.go                     # Embeds scripts into the binary
  .github/
    workflows/release.yml        # Builds and publishes releases on tag push
  Makefile
```

## Deployment (Admin Only)

This section covers the one-time shared infrastructure setup. This is done by an admin with `test-terraform` role access — not by individual developers.

### Shared Infrastructure (Tier 1)

Tier 1 creates the resources that all developer instances share: IAM instance profile, security group, S3 state bucket, and DynamoDB lock table. All resources are created in the existing Development VPC (configured in `terraform/shared/vpc.tf`). This only needs to be done once.

**AWS resources created:**

| Resource | Purpose |
|---|---|
| S3 bucket | Stores Terraform state for shared infrastructure |
| DynamoDB table | Prevents concurrent Terraform operations on shared infra |
| IAM role + instance profile (`clouddesktop-developer-instance`) | Grants EC2 instances access to SSM, CloudWatch, and ECR (read-only) |
| Security group (`clouddesktop-developer-instance`) | Zero inbound rules, all outbound allowed. Created in the Development VPC. Attached to every developer instance. |
| SSM parameters (shared: 4) | Stores SG ID, instance profile name, VPC ID, and subnet ID for the CLI to reference at provisioning time |
| SSM parameters (per-developer) | Stores S3 bucket name per developer (`/clouddesktop/developer/<name>/s3_bucket_name`) |
| S3 buckets (per-developer) | Created dynamically by `clouddesktop up`, named `clouddesktop-<name>`. Deleted by `clouddesktop destroy`. |

**Steps:**

```bash
# 1. Refresh MFA session
sts

# 2. Navigate to shared Terraform
cd terraform/shared

# 3. Bootstrap phase — comment out the S3 backend block in main.tf first,
#    since the S3 bucket doesn't exist yet
#
#    In main.tf, temporarily comment out:
#      backend "s3" { ... }

# 4. Initialize with local state
AWS_PROFILE=test-terraform terraform init

# 5. Apply — creates all shared resources including the S3 bucket
AWS_PROFILE=test-terraform terraform apply

# 6. Migrate state to S3 — uncomment the backend "s3" block in main.tf, then:
AWS_PROFILE=test-terraform terraform init -migrate-state

# 7. Verify SSM parameters were written
aws ssm get-parameter --name /clouddesktop/shared/security_group_id --profile test-developers
aws ssm get-parameter --name /clouddesktop/shared/instance_profile_name --profile test-developers
aws ssm get-parameter --name /clouddesktop/shared/vpc_id --profile test-developers
aws ssm get-parameter --name /clouddesktop/shared/subnet_id --profile test-developers
```

After this, shared infrastructure is done. The `test-terraform` profile is not needed again unless you modify `terraform/shared/`.

### Verifying the Deployment

After Tier 1 is applied, have one developer run through the full flow:

```bash
clouddesktop init      # Configure their settings
clouddesktop up        # Provision instance (~2 min infra + ~10 min bootstrap)
clouddesktop status    # Verify instance is running
clouddesktop ssh       # SSH in and verify tooling
clouddesktop down      # Stop instance
clouddesktop up        # Verify start/stop cycle (~30 sec)
```

Inside the instance, verify:
```bash
docker info                    # Docker running, no permission errors
java -version                  # Java 21
node --version                 # Node.js LTS
claude --version               # Claude Code CLI
docker pull <ACCOUNT_ID>.dkr.ecr.us-east-1.amazonaws.com/<any-service>  # ECR pull without login
```

### Tearing Down Shared Infrastructure

If you need to remove all shared infrastructure (not just a single developer's instance):

1. Ensure all developers have run `clouddesktop destroy --confirm` first
2. Navigate to `terraform/shared/` and run `AWS_PROFILE=test-terraform terraform destroy`

This removes the IAM role, security group, SSM parameters, S3 bucket, and DynamoDB table. The Development VPC and subnets are not affected.

## Troubleshooting

**"expired credentials" or "NoCredentialProviders"**
Run `sts` to refresh your MFA session, then retry.

**"no instance provisioned. Run 'clouddesktop up' first"**
You need to run `clouddesktop init` followed by `clouddesktop up` before using other commands.

**`clouddesktop up` hangs or times out**
Check your internet connection and AWS session. The first provision takes ~10 minutes for the bootstrap script to complete. You can SSH in separately to check progress:
```bash
ssh clouddesktop
tail -f /var/log/bootstrap-system.log
```

**SSH connection refused**
Ensure the SSM Session Manager plugin is installed (`session-manager-plugin`). Also verify the instance is running with `clouddesktop status`.

**S3 mount not working or `/home/ubuntu/s3` is empty**

If the mount didn't activate during provision (e.g., bootstrap was still running), run `clouddesktop up` again. The CLI will detect the missing mount and configure it via SSM. You can also verify manually on the instance:

```bash
mountpoint /home/ubuntu/s3       # Should say "is a mountpoint"
ls /home/ubuntu/s3               # Should list bucket contents
```

If the mount is not present, re-mount manually:
```bash
sudo mount-s3 --allow-delete --allow-overwrite --allow-other --uid 1000 --gid 1000 clouddesktop-<your-name> /home/ubuntu/s3
```

### SSH Agent Forwarding Issues

`clouddesktop` uses SSH agent forwarding to give the remote instance access to your local SSH keys. This means your private key never leaves your laptop -- the instance borrows it through the SSH connection. This is how `git clone` and `git push` work on the instance without storing secrets there.

**SSH key not forwarded to the instance**

Your SSH key isn't loaded into your local agent. Load it before connecting:

```bash
# On your laptop
ssh-add ~/.ssh/id_ed25519    # or whichever key you use for GitHub

# Verify it's loaded
ssh-add -l
# Should show your key fingerprint
```

Then reconnect to the instance. The key will be forwarded automatically because `ForwardAgent yes` is in the SSH config.

If your key requires a passphrase and you're on macOS, you can add it to Keychain so you don't have to re-enter it every time:
```bash
ssh-add --apple-use-keychain ~/.ssh/id_ed25519
```

**GitHub SSH authentication fails on the instance**

Your SSH key is loaded in the agent but not registered with GitHub. Add it:

1. Copy your public key: `cat ~/.ssh/id_ed25519.pub | pbcopy` (use your actual key path)
2. Go to [github.com/settings/keys](https://github.com/settings/keys)
3. Click "New SSH key", paste the public key, and save

Then reconnect to the instance. You can verify locally before connecting:
```bash
ssh -T git@github.com
# Should print: Hi <username>! You've successfully authenticated...
```

**Agent forwarding works on first connect but stops after reconnecting**

This can happen if your agent lost the key (e.g., after a laptop restart). Re-add it:
```bash
ssh-add ~/.ssh/id_ed25519    # or whichever key you use for GitHub
```

On macOS, keys added without `--apple-use-keychain` don't survive reboots.

**How to verify agent forwarding is working on the instance**

SSH into the instance and run:
```bash
# Check if the agent has forwarded keys
ssh-add -l

# Test GitHub access directly
ssh -T git@github.com
```

If `ssh-add -l` returns "The agent has no identities" or "Could not open a connection to your authentication agent", agent forwarding isn't working. Check that:
1. Your key is loaded locally (`ssh-add -l` on your laptop)
2. You connected via the `clouddesktop` SSH host (which has `ForwardAgent yes`)
3. You didn't connect through an intermediate jump host that strips agent forwarding
