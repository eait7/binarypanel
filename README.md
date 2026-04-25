# BinaryPanel — The Go-Native Infrastructure Ecosystem

BinaryPanel is a fully Go-native, composable server management ecosystem that replaces traditional control panels. It orchestrates three best-in-class open-source tools—all written in Go—unified by a custom, lightweight dashboard:

1. **Caddy**: High-performance reverse proxy and automatic SSL.
2. **FileBrowser**: Beautiful web-based storage and file management.
3. **Portainer**: Visual orchestration and deployment manager.
4. **BinaryPanel Dashboard**: Fast, zero-dependency SPA connecting it all via APIs.

---

## ✨ Features

### 🌐 Domain Management
- **Add, edit, and remove domains** with a clean UI — reverse proxy or static file server.
- **Automatic SSL** via Caddy's built-in ACME/Let's Encrypt integration.
- **Backup & Restore** — download a domain's deployment as a `.zip` or upload one to restore.
- **Process Restart** — bounce backend services mapped to a domain directly from the dashboard.

### 🛡️ Domain Security Suite
Per-domain security hardening with real-time Caddy API integration:

- **Security Score** (0–100) per domain with color-coded badges:
  - 🟢 ≥ 80 — Well Protected
  - 🟡 40–79 — Partial Protection
  - 🔴 < 40 — Unprotected
- **One-click Presets** — Apply **Strict**, **Balanced**, or **Minimal** security profiles instantly.
- **Security Headers** (applied live, no restart needed):
  - HSTS (with max-age, includeSubDomains, preload)
  - X-Frame-Options (DENY / SAMEORIGIN)
  - X-Content-Type-Options: nosniff
  - Referrer-Policy
  - Permissions-Policy
  - Content-Security-Policy (CSP)
- **IP Access Control** — Per-domain IP whitelist and blacklist with automatic Caddy route injection.
- **Global IP Blocklist** — Block IPs across all domains from the Settings page.
- **Persistent** — All security configs saved to `/data/security.json` and auto-reapplied on container restart.

### 📊 Observability & Diagnostics
- **Structured Error Logging** — In-memory ring buffer + persistent file-based logging to `/data/binarypanel.log`.
- **Diagnostic Engine** — Automatically maps raw network errors (DNS, timeout, connection refused) to user-friendly fix suggestions.
- **Log Viewer** — Color-coded error log modal with level filtering (All / Errors / Warnings / Info) and live refresh.

### ⚡ System Management
- **One-Click Stack Reboot** — Restart all BinaryPanel containers (Caddy, Dashboard, FileBrowser) to resolve DNS failures or stale connections, with automatic reconnection polling.
- **Self-Compiling Updates** — Pull the latest code from GitHub and rebuild the dashboard container natively from the Settings page.
- **Container Management** — Start, stop, restart, and view logs for all Docker containers.
- **System Monitoring** — Real-time CPU, memory, disk usage gauges, load averages, and uptime stats.

### 🏪 App Store
- **1-Click Installers** — Deploy applications like BinaryCMS directly from the dashboard.

---

## 🚀 One-Command Installation

The entire stack can be installed on a fresh Ubuntu/Debian server using a single command. Open your terminal and run:

```bash
curl -sL https://raw.githubusercontent.com/eait7/binarypanel/main/install.sh | sudo bash
```

The script will automatically:
1. Install Docker, Docker Compose, and Git if they aren't already present.
2. Clone this repository into `/opt/binarypanel`.
3. Add your user to the `docker` group.
4. Orchestrate and launch all necessary services via Docker Compose.

*Note: After running this script for the first time, you must close your terminal and open a new one to apply the `docker` group changes.*

## 🌟 Accessing the Ecosystem

All components are securely accessible out of the box. 

- **BinaryPanel Dashboard**: `http://<your-ip>:9000`
  - *Login: `admin` / `admin`*
- **FileBrowser**: `http://<your-ip>:8090`
  - *Login: `admin` / `Admin123456!`* (Due to standard 12-character constraint)
- **Portainer**: `https://<your-ip>:9443`
  - *Create your own initial password upon first visit.*
- **Caddy (Web)**: `http://<your-ip>:80` & `https://<your-ip>:443`

> **IMPORTANT**: You should immediately log into all services and change their default passwords!

## 🔧 Architecture Overview

- **Zero-Bloat Frontend**: Built with pure HTML/CSS/JS (no heavy framework bundles), utilizing advanced glassmorphism design and optimized for blazing-fast speed.
- **RESTful Orchestration**: The BinaryPanel Dashboard securely communicates with Caddy's REST API and Docker's local socket without requiring heavy external SDKs.
- **Live Security Injection**: Security headers and IP restrictions are applied in real-time via Caddy's JSON admin API — no server restart required.
- **Rootless Compatibility**: Docker configurations support native Unix sockets and rootless setups where user groups possess appropriate permissions.

## 📜 Licensing and Dependencies

BinaryPanel is entirely **Free and Open-Source** under the MIT license. 
Rest assured, there are **no enterprise paywalls**. Every dependency we use is specifically chosen because it permits commercial and non-commercial redistribution with no strings attached:
- **Caddy**: Apache 2.0 License
- **FileBrowser**: Apache 2.0 License
- **Portainer CE**: zlib License

Enjoy your modern, native, and fully composable control panel!
