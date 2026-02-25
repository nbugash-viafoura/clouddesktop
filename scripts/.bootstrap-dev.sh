#!/bin/bash
# CloudDesktop Developer Bootstrap Script
# Run manually by the developer after first SSH connection
# Make executable with: chmod +x .bootstrap-dev.sh
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
  log "Make sure your local SSH agent has your key loaded (ssh-add ~/.ssh/viafoura_dev)"
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
