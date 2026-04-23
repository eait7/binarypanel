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

docker compose up -d --build

echo ""
echo "═══════════════════════════════════════════"
echo "  ✅ BinaryPanel is running!"
echo "  Dashboard:    http://localhost:9000"
echo "  FileBrowser:  http://localhost:8090"
echo "  Portainer:    https://localhost:9443"
echo "  Caddy:        http://localhost:80"
echo ""
echo "  Login: admin / admin"
echo "  (Change in .env file)"
echo "═══════════════════════════════════════════"
echo ""
echo "⚠️  Log out and back in (or run 'newgrp docker')"
echo "   for docker commands to work without sudo."
