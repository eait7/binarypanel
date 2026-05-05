#!/bin/bash
# BinaryPanel Setup Script — Run with: sudo bash setup.sh
# This script adds your user to the docker group and starts BinaryPanel

set -e

USER_NAME="${SUDO_USER:-$(whoami)}"
echo "🔧 Adding $USER_NAME to the docker group..."
usermod -aG docker "$USER_NAME"
echo "✅ User added to docker group"

echo ""
echo "🚀 Starting BinaryPanel..."
cd "$(dirname "$0")"

# Configure environment variables
if [ ! -f .env ]; then
    echo "📄 Generating default .env file..."
    cp .env.example .env
fi

# ── Generate self-signed TLS certificate for the BinaryPanel admin panel ────
# This cert is used by Caddy to serve https://YOUR-SERVER-IP:8443.
# It includes all detected server IPs as Subject Alternative Names (SANs)
# so the browser can connect without a "wrong host" error.
echo "🔐 Generating self-signed TLS certificate for panel HTTPS..."

CERT_DIR="./caddy_config"
CERT_FILE="$CERT_DIR/panel.crt"
KEY_FILE="$CERT_DIR/panel.key"

# Collect all non-loopback IPv4 addresses
SERVER_IPS=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -E '^[0-9]+\.' | head -5 | tr '\n' ',')
# Always include loopback
SAN="IP:127.0.0.1,DNS:localhost"
if [ -n "$SERVER_IPS" ]; then
    # Build IP:x.x.x.x entries
    for ip in $(echo "$SERVER_IPS" | tr ',' ' '); do
        [ -n "$ip" ] && SAN="IP:$ip,$SAN"
    done
fi

# Generate the cert using openssl (available on any Linux system)
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
    -days 3650 -nodes \
    -keyout "$KEY_FILE" \
    -out "$CERT_FILE" \
    -subj "/O=BinaryPanel/CN=BinaryPanel Admin" \
    -addext "subjectAltName=$SAN" \
    2>/dev/null

echo "✅ TLS certificate generated (valid 10 years, IPs: $SERVER_IPS 127.0.0.1)"
# ────────────────────────────────────────────────────────────────────────────

docker compose up -d --build

# Detect server IP for display
DISPLAY_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -z "$DISPLAY_IP" ] && DISPLAY_IP="YOUR-SERVER-IP"

echo ""
echo "═══════════════════════════════════════════"
echo "  ✅ BinaryPanel is running!"
echo ""
echo "  🔒 Panel (HTTPS):  https://$DISPLAY_IP:8443"
echo "     → Accept the browser's security warning (self-signed cert)"
echo "     → For a proper SSL cert, set PANEL_DOMAIN= in .env"
echo ""
echo "  FileBrowser:   http://localhost:8090"
echo "  Portainer:     https://localhost:9443"
echo ""
echo "  Login: admin / admin"
echo "  (Change credentials in .env file)"
echo "═══════════════════════════════════════════"
echo ""
echo "⚠️  Log out and back in (or run 'newgrp docker')"
echo "   for docker commands to work without sudo."
