# clouddesktop — Cloud Desktop CLI

A self-service CLI tool for provisioning and managing personal cloud development environments on AWS EC2. Each developer gets their own instance with Docker, Java 21, Node.js, and all the tooling needed to run Viafoura services remotely.

## Why

Local developer machines allocate 8 CPU + 16 GB RAM to Colima(if you're using MacOS), leaving the laptop in survival mode for the IDE and browser. `clouddesktop` moves that compute to an EC2 instance where Docker runs natively (no VM layer, no hypervisor overhead). The local machine only runs the IDE frontend — all builds, containers, and tests execute on the remote instance.

## How It Works

- **Provision**: `clouddesktop up` creates an EC2 instance via the AWS SDK on first run (~2 minutes)
- **Start/Stop**: subsequent `clouddesktop up` and `clouddesktop down` start and stop the instance (~30 seconds)
- **Persist**: stopping an instance preserves the EBS volume — repos, Docker image cache, Gradle/npm caches, shell history all survive across sessions
- **Connect**: SSH tunneled through AWS SSM Session Manager (zero open inbound ports). JetBrains Gateway and VS Code Remote SSH work transparently via the SSH config entry `clouddesktop` writes automatically
- **Destroy**: `clouddesktop destroy --confirm` terminates the instance and deletes all data. Requires explicit confirmation.

### Cost

Compute cost is incurred only while the instance is running. Stopped instances only incur EBS storage cost.

| Instance type | Running (10h/day, 22 days/month) | Stopped (EBS only) | Monthly estimate |
|---|---|---|---|
| `m7i.xlarge` (4 vCPU, 16 GB) | ~$44 | ~$8 | ~$52 |
| `m7i.2xlarge` (8 vCPU, 32 GB) | ~$89 | ~$16 | ~$105 |

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

Verify your AWS profile is configured. `clouddesktop` uses the `test-developers` profile by default:

```bash
aws sts get-caller-identity --profile test-developers
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

Download the latest release for your platform from [GitHub Releases](https://github.com/nbugash-viafoura/clouddesktop/releases/latest):

```bash
# macOS Apple Silicon
curl -L https://github.com/nbugash-viafoura/clouddesktop/releases/latest/download/clouddesktop-<version>-darwin-arm64.tar.gz | tar xz
sudo mv clouddesktop /usr/local/bin/

# macOS Intel
curl -L https://github.com/nbugash-viafoura/clouddesktop/releases/latest/download/clouddesktop-<version>-darwin-amd64.tar.gz | tar xz
sudo mv clouddesktop /usr/local/bin/

# Linux
curl -L https://github.com/nbugash-viafoura/clouddesktop/releases/latest/download/clouddesktop-<version>-linux-amd64.tar.gz | tar xz
sudo mv clouddesktop /usr/local/bin/
```

Replace `<version>` with the release tag (e.g., `v1.0.0`). Verify with `clouddesktop --version`.

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
- **AWS profile** — default: `test-terraform`
- **Instance type** — default: `m7i.xlarge` (4 vCPU, 16 GB)
- **SSH public key path** — default: `~/.ssh/id_ed25519.pub`

Region is fixed to `us-east-1`.

Writes configuration to `~/.clouddesktop/config.yaml`.

### Provision and Start

```bash
clouddesktop up
```

On first run, this provisions an EC2 instance (~2 minutes for infrastructure, ~10 minutes for bootstrap). On subsequent runs, it starts the existing stopped instance (~30 seconds).

After the instance is running, `clouddesktop up` automatically writes an SSH config entry to `~/.ssh/config`:

```
Host clouddesktop
  HostName <instance-id>
  User ubuntu
  IdentityFile ~/.ssh/<your-key>
  ForwardAgent yes
  ProxyCommand aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters portNumber=%p --profile test-developers --region us-east-1
  ServerAliveInterval 60
  ServerAliveCountMax 3
  StrictHostKeyChecking accept-new
```

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

### Resize

```bash
clouddesktop resize m7i.2xlarge
```

Changes the instance type. Stops the instance if running, applies the change, then restarts. Use this if CloudWatch metrics show you need more resources.

### Destroy

```bash
clouddesktop destroy --confirm
```

Permanently terminates the instance and deletes all associated resources (EBS volume, key pair). The `--confirm` flag is required. After destroying, run `clouddesktop init` to start fresh.

## Commands Reference

| Command | Description |
|---|---|
| `clouddesktop init` | One-time setup: configure AWS profile, instance type, SSH key |
| `clouddesktop up` | Start instance (or provision on first run) |
| `clouddesktop down` | Stop instance (data preserved) |
| `clouddesktop status` | Show instance state, IP, uptime, metrics |
| `clouddesktop ssh` | Open SSH session to instance |
| `clouddesktop resize <type>` | Change instance type (e.g., `m7i.2xlarge`) |
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
                                         +---------------------------+
                                         | IAM Instance Profile      |
                                         |   - SSM access            |
                                         |   - ECR read-only         |
                                         |   - CloudWatch metrics    |
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
    cli/                         # One file per command
    aws/                         # AWS SDK clients (EC2, SSM, CloudWatch, STS, provisioner)
    config/                      # ~/.clouddesktop/config.yaml read/write
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
| SSM parameters (4) | Stores SG ID, instance profile name, VPC ID, and subnet ID for the CLI to reference at provisioning time |

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
