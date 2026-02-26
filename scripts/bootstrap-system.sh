#!/bin/bash
# CloudDesktop System Bootstrap Script
# Runs as root via EC2 user_data on first boot
# Make executable with: chmod +x bootstrap-system.sh

set -euo pipefail

exec > >(tee -a /var/log/bootstrap-system.log) 2>&1

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log "Starting CloudDesktop system bootstrap..."

log "Step 1: Updating system packages and installing base utilities..."
apt-get update -y
DEBIAN_FRONTEND=noninteractive apt-get install -y \
  git curl wget jq make tmux zsh zip unzip \
  build-essential ca-certificates gnupg lsb-release \
  apt-transport-https software-properties-common \
  unattended-upgrades

log "Step 2: Installing GitHub CLI from official repository..."
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list
apt-get update -y
apt-get install -y gh

log "Step 3: Installing Docker Engine..."
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" > /etc/apt/sources.list.d/docker.list
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

systemctl enable docker
systemctl start docker

log "Adding ubuntu user to docker group..."
usermod -aG docker ubuntu

log "Step 4: Installing and configuring ECR credential helper..."
apt-get install -y amazon-ecr-credential-helper

IMDS_TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 60")
IDENTITY_DOC=$(curl -s -H "X-aws-ec2-metadata-token: ${IMDS_TOKEN}" http://169.254.169.254/latest/dynamic/instance-identity/document)
AWS_ACCOUNT_ID=$(echo "${IDENTITY_DOC}" | jq -r '.accountId')
AWS_REGION=$(echo "${IDENTITY_DOC}" | jq -r '.region')
log "Detected AWS Account: ${AWS_ACCOUNT_ID}, Region: ${AWS_REGION}"
mkdir -p /home/ubuntu/.docker
cat > /home/ubuntu/.docker/config.json <<EOF
{
  "credHelpers": {
    "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com": "ecr-login"
  }
}
EOF
chown -R ubuntu:ubuntu /home/ubuntu/.docker

log "Step 5: Installing AWS CLI v2..."
curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o /tmp/awscliv2.zip
unzip -q /tmp/awscliv2.zip -d /tmp
/tmp/aws/install
rm -rf /tmp/aws /tmp/awscliv2.zip

log "Step 6: Installing Kubernetes tooling (kubectl, helm, k3d, istioctl, kubectl-argo-rollouts, helm-unittest)..."

log "Installing kubectl..."
curl -fsSL "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl
chmod 755 /usr/local/bin/kubectl

log "Installing Helm..."
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

log "Installing k3d..."
curl -fsSL https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

log "Installing istioctl..."
curl -fsSL https://istio.io/downloadIstio | sh -
mv istio-*/bin/istioctl /usr/local/bin/istioctl
chmod 755 /usr/local/bin/istioctl
rm -rf istio-*/

log "Installing kubectl-argo-rollouts..."
curl -fsSL -o /tmp/kubectl-argo-rollouts-linux-amd64 \
  "https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64"
chmod 755 /tmp/kubectl-argo-rollouts-linux-amd64
mv /tmp/kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts

log "Installing Helm unittest plugin..."
su - ubuntu -c 'helm plugin install https://github.com/helm-unittest/helm-unittest'

log "Verifying Kubernetes tooling..."
kubectl version --client --output=yaml | head -5
helm version --short
k3d version
istioctl version --remote=false
kubectl-argo-rollouts version --short
su - ubuntu -c 'helm plugin list' | grep unittest

log "Step 7: Installing SDKMAN and Java 21 (Amazon Corretto)..."
su - ubuntu -c '
  set -eo pipefail
  export SDKMAN_DIR="$HOME/.sdkman"
  curl -fsSL "https://get.sdkman.io" | bash
  sed -i "s/sdkman_auto_answer=false/sdkman_auto_answer=true/" "$SDKMAN_DIR/etc/config"
  sed -i "s/sdkman_colour_enable=true/sdkman_colour_enable=false/" "$SDKMAN_DIR/etc/config"
  echo "sdkman_auto_env=true" >> "$SDKMAN_DIR/etc/config"
  bash -c "source $SDKMAN_DIR/bin/sdkman-init.sh && sdk install java 21.0.10-amzn && sdk default java 21.0.10-amzn"
'

log "Step 8: Installing fnm and Node.js LTS..."
su - ubuntu -c '
  set -euo pipefail
  curl -fsSL https://fnm.vercel.app/install | bash
  export FNM_PATH="$HOME/.local/share/fnm"
  export PATH="$FNM_PATH:$PATH"
  eval "$(fnm env)"
  fnm install --lts
  fnm use lts-latest
  fnm default lts-latest
  npm install -g pnpm yarn
'

log "Step 9: Installing Claude Code CLI..."
su - ubuntu -c '
  set -euo pipefail
  export FNM_PATH="$HOME/.local/share/fnm"
  export PATH="$FNM_PATH:$PATH"
  eval "$(fnm env)"
  npm install -g @anthropic-ai/claude-code
'

log "Step 10: Installing zsh and oh-my-zsh..."
chsh -s "$(which zsh)" ubuntu

su - ubuntu -c '
  set -euo pipefail
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended
'

log "Configuring .bashrc with fnm and SDKMAN initialization..."
cat >> /home/ubuntu/.bashrc <<'BASHEOF'

# fnm
export FNM_PATH="$HOME/.local/share/fnm"
if [ -d "$FNM_PATH" ]; then
  export PATH="$FNM_PATH:$PATH"
  eval "$(fnm env --use-on-cd)"
fi

# SDKMAN
export SDKMAN_DIR="$HOME/.sdkman"
[[ -s "$HOME/.sdkman/bin/sdkman-init.sh" ]] && source "$HOME/.sdkman/bin/sdkman-init.sh"

# Auto-switch to zsh for interactive sessions (SSM drops into bash)
if [[ $- == *i* ]] && command -v zsh &>/dev/null; then
  exec zsh -l
fi
BASHEOF
chown ubuntu:ubuntu /home/ubuntu/.bashrc

log "Configuring .zshrc with fnm and SDKMAN initialization..."
cat >> /home/ubuntu/.zshrc <<'ZSHEOF'

# fnm
export FNM_PATH="$HOME/.local/share/fnm"
if [ -d "$FNM_PATH" ]; then
  export PATH="$FNM_PATH:$PATH"
  eval "$(fnm env --use-on-cd)"
fi

# SDKMAN
export SDKMAN_DIR="$HOME/.sdkman"
[[ -s "$HOME/.sdkman/bin/sdkman-init.sh" ]] && source "$HOME/.sdkman/bin/sdkman-init.sh"
ZSHEOF
chown ubuntu:ubuntu /home/ubuntu/.zshrc

log "Step 11: Verifying SSM Agent installation and status..."
if ! systemctl is-active --quiet amazon-ssm-agent; then
  log "SSM Agent not active, installing via snap..."
  snap install amazon-ssm-agent --classic
  systemctl enable snap.amazon-ssm-agent.amazon-ssm-agent.service
  systemctl start snap.amazon-ssm-agent.amazon-ssm-agent.service
else
  log "SSM Agent already active."
fi

log "Step 12: Installing CloudWatch Agent..."
wget -q https://s3.amazonaws.com/amazoncloudwatch-agent/ubuntu/amd64/latest/amazon-cloudwatch-agent.deb -O /tmp/amazon-cloudwatch-agent.deb
dpkg -i /tmp/amazon-cloudwatch-agent.deb
rm /tmp/amazon-cloudwatch-agent.deb

log "Step 13: Hardening SSH configuration..."
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#*ChallengeResponseAuthentication.*/ChallengeResponseAuthentication no/' /etc/ssh/sshd_config
systemctl restart ssh

log "Step 14: Installing clouddesktop auto-stop service..."

cat > /usr/local/bin/clouddesktop-autostop <<'AUTOSTOP_SCRIPT'
#!/bin/bash
# clouddesktop-autostop: Shuts down the instance after 4 hours of SSH inactivity.
# Invoked every 15 minutes by clouddesktop-autostop.timer.

set -euo pipefail

SENTINEL="/var/log/bootstrap-system-complete"
COUNTER_FILE="/var/run/clouddesktop-idle-count"
IDLE_THRESHOLD=16
LOG_TAG="clouddesktop-autostop"

log() {
  logger -t "$LOG_TAG" "$*"
}

if [[ ! -f "$SENTINEL" ]]; then
  log "Bootstrap not yet complete. Skipping idle check."
  exit 0
fi

ssh_session_count=$(who | grep -c "pts/" || true)

if [[ "$ssh_session_count" -gt 0 ]]; then
  log "Active SSH sessions detected ($ssh_session_count). Resetting idle counter."
  echo 0 > "$COUNTER_FILE"
  exit 0
fi

current_count=0
if [[ -f "$COUNTER_FILE" ]]; then
  current_count=$(cat "$COUNTER_FILE")
fi

new_count=$((current_count + 1))
echo "$new_count" > "$COUNTER_FILE"

log "No active SSH sessions. Idle check $new_count/$IDLE_THRESHOLD."

if [[ "$new_count" -ge "$IDLE_THRESHOLD" ]]; then
  log "Idle threshold reached ($IDLE_THRESHOLD consecutive checks, 4 hours). Initiating shutdown."
  shutdown -h now
fi
AUTOSTOP_SCRIPT

chmod 755 /usr/local/bin/clouddesktop-autostop

cat > /etc/systemd/system/clouddesktop-autostop.service <<'AUTOSTOP_SERVICE'
[Unit]
Description=CloudDesktop auto-stop idle check
After=network.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/clouddesktop-autostop
StandardOutput=journal
StandardError=journal
AUTOSTOP_SERVICE

cat > /etc/systemd/system/clouddesktop-autostop.timer <<'AUTOSTOP_TIMER'
[Unit]
Description=CloudDesktop auto-stop timer (runs every 15 minutes)

[Timer]
OnBootSec=15min
OnUnitActiveSec=15min

[Install]
WantedBy=timers.target
AUTOSTOP_TIMER

systemctl daemon-reload
systemctl enable clouddesktop-autostop.timer
systemctl start clouddesktop-autostop.timer

log "Step 15: Writing .bootstrap-dev.sh for developer setup..."
cat > /home/ubuntu/.bootstrap-dev.sh <<'DEVSCRIPT'
#!/bin/bash
# CloudDesktop Developer Bootstrap Script
# Run manually by the developer after first SSH connection
#
# Prerequisites:
#   - SSH agent forwarding must be enabled (ForwardAgent yes in SSH config)
#   - Your local SSH key must be added to your GitHub account

set -euo pipefail

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log "Starting CloudDesktop developer bootstrap..."
log "This script runs as the ubuntu user and configures your personal development environment."
echo ""

log "Step 1: Verifying SSH agent forwarding and GitHub access..."
if ! ssh-add -l &>/dev/null; then
  log "ERROR: No SSH keys available via agent forwarding."
  log "Make sure your local SSH agent has your key loaded (ssh-add <your-key>)"
  log "and that you connected with agent forwarding (ssh -A clouddesktop)."
  exit 1
fi
log "SSH agent has forwarded keys."

if ssh -T git@github.com 2>&1 | grep -q "successfully authenticated"; then
  log "GitHub SSH connection verified via agent forwarding."
else
  log "ERROR: GitHub SSH authentication failed."
  log "Make sure your SSH key is added to your GitHub account at github.com/settings/keys"
  log "Debug: Run 'ssh-add -l' to see loaded keys, 'ssh -vT git@github.com' to debug."
  exit 1
fi

log "Step 2: Configuring Testcontainers..."
cat > "$HOME/.testcontainers.properties" <<'EOF'
docker.host=unix:///var/run/docker.sock
ryuk.disabled=true
host.override=127.0.0.1
EOF
log "Testcontainers configuration written to ~/.testcontainers.properties"

log "Step 3: Checking for optional dotfiles repository..."
if [ -n "${DOTFILES_REPO:-}" ]; then
  log "DOTFILES_REPO set to: $DOTFILES_REPO"
  if [ ! -d "$HOME/.dotfiles" ]; then
    log "Cloning dotfiles from $DOTFILES_REPO..."
    git clone "$DOTFILES_REPO" "$HOME/.dotfiles"
    if [ -f "$HOME/.dotfiles/install.sh" ]; then
      log "Running dotfiles install script..."
      bash "$HOME/.dotfiles/install.sh"
    else
      log "No install.sh found in dotfiles repo. Clone complete at ~/.dotfiles"
      log "You may need to manually symlink or install your dotfiles."
    fi
  else
    log "Dotfiles directory already exists at ~/.dotfiles. Skipping clone."
  fi
else
  log "DOTFILES_REPO environment variable not set. Skipping dotfiles installation."
  log "To install dotfiles later, run: DOTFILES_REPO=<your-repo-url> bash ~/.bootstrap-dev.sh"
fi

echo ""
echo "=================================================================================="
echo "Developer bootstrap complete!"
echo "=================================================================================="
echo ""
echo "Next steps:"
echo "  1. Clone your repositories into ~/Development/"
echo "     Example: git clone git@github.com:Viafoura/your-repo.git ~/Development/your-repo"
echo ""
echo "  2. Start dependency services using Docker Compose:"
echo "     docker compose -f docker-compose.dependencies.yaml up -d"
echo ""
echo "  3. Connect your IDE:"
echo "     - JetBrains Gateway: Use the 'clouddesktop' SSH host"
echo "     - VS Code Remote SSH: Use the 'clouddesktop' SSH host"
echo ""
echo "  4. Verify your development environment:"
echo "     java -version    # Should show Amazon Corretto 21"
echo "     node -v          # Should show Node.js LTS"
echo "     docker ps        # Should show Docker is accessible"
echo ""
echo "Happy coding!"
echo "=================================================================================="
DEVSCRIPT
chmod 555 /home/ubuntu/.bootstrap-dev.sh
chown root:root /home/ubuntu/.bootstrap-dev.sh

log "Step 16: Marking bootstrap as complete..."
touch /var/log/bootstrap-system-complete

log "Bootstrap complete. Instance is ready for developer-specific setup."
log "Developer should run ~/.bootstrap-dev.sh after first SSH connection."
