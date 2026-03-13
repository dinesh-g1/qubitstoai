#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# qubitstoai.com — VPS bootstrap script
# Run this ONCE on a fresh Ubuntu 22.04 / 24.04 VPS as root.
# After this, all future deploys go through GitHub Actions automatically.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/qubitstoai/site/main/infra/bootstrap.sh | bash
#   OR copy this file to your VPS and run: bash bootstrap.sh
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

REPO="https://github.com/dinesh-g1/qubitstoai.git"
APP_DIR="/srv/qubitstoai"
DEPLOY_USER="deploy"

echo "──────────────────────────────────────────"
echo " qubitstoai.com bootstrap"
echo "──────────────────────────────────────────"

# 1. System update
apt-get update -y && apt-get upgrade -y
apt-get install -y git curl ufw fail2ban

# 2. Install Docker
curl -fsSL https://get.docker.com | sh
systemctl enable docker

# 3. Firewall
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow 80
ufw allow 443
ufw --force enable

# 4. Create deploy user (for CI/CD SSH)
if ! id "$DEPLOY_USER" &>/dev/null; then
  useradd -m -s /bin/bash "$DEPLOY_USER"
  usermod -aG docker "$DEPLOY_USER"
  mkdir -p /home/$DEPLOY_USER/.ssh
  chmod 700 /home/$DEPLOY_USER/.ssh
  # Paste your GitHub Actions public key here:
  echo "PASTE_YOUR_GITHUB_ACTIONS_PUBLIC_KEY_HERE" > /home/$DEPLOY_USER/.ssh/authorized_keys
  chmod 600 /home/$DEPLOY_USER/.ssh/authorized_keys
  chown -R $DEPLOY_USER:$DEPLOY_USER /home/$DEPLOY_USER/.ssh
fi

# 5. Clone repo
mkdir -p "$APP_DIR"
if [ ! -d "$APP_DIR/.git" ]; then
  git clone "$REPO" "$APP_DIR"
fi
chown -R $DEPLOY_USER:$DEPLOY_USER "$APP_DIR"

# 6. Create .env from example
if [ ! -f "$APP_DIR/.env" ]; then
  cp "$APP_DIR/.env.example" "$APP_DIR/.env"
  # Generate a strong DB password automatically
  DB_PASS=$(openssl rand -base64 32 | tr -d '/+=')
  sed -i "s/change_me_to_a_strong_random_password/$DB_PASS/" "$APP_DIR/.env"
  echo ""
  echo "  DB_PASSWORD has been set to: $DB_PASS"
  echo "  (saved in $APP_DIR/.env)"
  echo ""
fi

# 7. Start the stack
cd "$APP_DIR"
docker compose up -d --build

echo ""
echo "──────────────────────────────────────────"
echo " Bootstrap complete!"
echo ""
echo " Next steps:"
echo " 1. Point your DNS A record for qubitstoai.com → $(curl -s ifconfig.me)"
echo " 2. Wait for DNS to propagate (up to 24h)"
echo " 3. Caddy will auto-provision your SSL cert"
echo " 4. Add these GitHub Actions secrets:"
echo "    VPS_HOST = $(curl -s ifconfig.me)"
echo "    VPS_USER = $DEPLOY_USER"
echo "    VPS_SSH_KEY = (your deploy private key)"
echo ""
echo " Site will be live at: https://qubitstoai.com"
echo "──────────────────────────────────────────"
