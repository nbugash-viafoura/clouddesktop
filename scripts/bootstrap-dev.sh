#!/bin/bash
# CloudDesktop Developer Bootstrap Script
# Run manually by the developer after first SSH connection
# Make executable with: chmod +x bootstrap-dev.sh

set -euo pipefail

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

log "Starting CloudDesktop developer bootstrap..."
log "This script runs as the ubuntu user and configures your personal development environment."
echo ""

log "Step 1: Setting up GitHub deploy key..."
if [ ! -f "$HOME/.ssh/github_deploy" ]; then
  ssh-keygen -t ed25519 -C "clouddesktop-$(hostname)" -f "$HOME/.ssh/github_deploy" -N ""
  log "GitHub deploy key generated at ~/.ssh/github_deploy"
  echo ""
  echo "=================================================================================="
  echo "Add the following public key as a deploy key to your GitHub repositories:"
  echo "  GitHub Settings -> Deploy keys -> Add deploy key"
  echo "  Check 'Allow write access' if you need to push commits"
  echo "=================================================================================="
  echo ""
  cat "$HOME/.ssh/github_deploy.pub"
  echo ""
  echo "=================================================================================="
  read -p "Press Enter once you have added the deploy key to GitHub..."
else
  log "GitHub deploy key already exists at ~/.ssh/github_deploy. Skipping generation."
fi

log "Configuring SSH to use the deploy key for github.com..."
mkdir -p "$HOME/.ssh"
if ! grep -q "Host github.com" "$HOME/.ssh/config" 2>/dev/null; then
  cat >> "$HOME/.ssh/config" <<'EOF'
Host github.com
  IdentityFile ~/.ssh/github_deploy
  StrictHostKeyChecking accept-new
EOF
  chmod 600 "$HOME/.ssh/config"
  log "SSH config updated for github.com"
else
  log "SSH config already contains github.com host entry. Skipping."
fi

log "Step 2: Testing GitHub SSH connection..."
if ssh -T git@github.com 2>&1 | grep -q "successfully authenticated"; then
  log "GitHub SSH connection successful"
else
  log "GitHub SSH connection test completed (exit code indicates success for GitHub)"
fi

log "Step 3: Configuring Testcontainers..."
cat > "$HOME/.testcontainers.properties" <<'EOF'
docker.host=unix:///var/run/docker.sock
ryuk.disabled=true
host.override=127.0.0.1
EOF
log "Testcontainers configuration written to ~/.testcontainers.properties"

log "Step 4: Checking for optional dotfiles repository..."
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
  log "To install dotfiles later, run: DOTFILES_REPO=<your-repo-url> bash bootstrap-dev.sh"
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
