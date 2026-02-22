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
  git curl wget jq make tmux zsh unzip \
  build-essential ca-certificates gnupg lsb-release \
  apt-transport-https software-properties-common \
  unattended-upgrades

log "Step 2: Installing Docker Engine..."
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

log "Step 3: Installing and configuring ECR credential helper..."
apt-get install -y amazon-ecr-credential-helper

mkdir -p /home/ubuntu/.docker
cat > /home/ubuntu/.docker/config.json <<'EOF'
{
  "credHelpers": {
    "218894879100.dkr.ecr.us-east-1.amazonaws.com": "ecr-login"
  }
}
EOF
chown -R ubuntu:ubuntu /home/ubuntu/.docker

log "Step 4: Installing AWS CLI v2..."
curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o /tmp/awscliv2.zip
unzip -q /tmp/awscliv2.zip -d /tmp
/tmp/aws/install
rm -rf /tmp/aws /tmp/awscliv2.zip

log "Step 5: Installing SDKMAN and Java 21 (Amazon Corretto)..."
su - ubuntu -c '
  set -euo pipefail
  curl -fsSL "https://get.sdkman.io" | bash
  source "$HOME/.sdkman/bin/sdkman-init.sh"
  sdk install java 21.0.5-amzn
  sdk default java 21.0.5-amzn
  echo "sdkman_auto_env=true" >> "$HOME/.sdkman/etc/config"
'

log "Step 6: Installing fnm and Node.js LTS..."
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

log "Step 7: Installing Claude Code CLI..."
su - ubuntu -c '
  set -euo pipefail
  export FNM_PATH="$HOME/.local/share/fnm"
  export PATH="$FNM_PATH:$PATH"
  eval "$(fnm env)"
  npm install -g @anthropic-ai/claude-code
'

log "Step 8: Installing zsh and oh-my-zsh..."
chsh -s "$(which zsh)" ubuntu

su - ubuntu -c '
  set -euo pipefail
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended
'

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

log "Step 9: Verifying SSM Agent installation and status..."
if ! systemctl is-active --quiet amazon-ssm-agent; then
  log "SSM Agent not active, installing via snap..."
  snap install amazon-ssm-agent --classic
  systemctl enable snap.amazon-ssm-agent.amazon-ssm-agent.service
  systemctl start snap.amazon-ssm-agent.amazon-ssm-agent.service
else
  log "SSM Agent already active."
fi

log "Step 10: Installing CloudWatch Agent..."
wget -q https://s3.amazonaws.com/amazoncloudwatch-agent/ubuntu/amd64/latest/amazon-cloudwatch-agent.deb -O /tmp/amazon-cloudwatch-agent.deb
dpkg -i /tmp/amazon-cloudwatch-agent.deb
rm /tmp/amazon-cloudwatch-agent.deb

log "Step 11: Hardening SSH configuration..."
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/^#*ChallengeResponseAuthentication.*/ChallengeResponseAuthentication no/' /etc/ssh/sshd_config
systemctl restart sshd

log "Step 12: Marking bootstrap as complete..."
touch /var/log/bootstrap-system-complete

log "Bootstrap complete. Instance is ready for developer-specific setup."
log "Developer should run bootstrap-dev.sh after first SSH connection."
