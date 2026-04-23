# BinaryPanel — The Go-Native Infrastructure Ecosystem

BinaryPanel is a fully Go-native, composable server management ecosystem that replaces traditional control panels. It orchestrates three best-in-class open-source tools—all written in Go—unified by a custom, lightweight dashboard:

1. **Caddy**: High-performance reverse proxy and automatic SSL.
2. **FileBrowser**: Beautiful web-based storage and file management.
3. **Portainer**: Visual orchestration and deployment manager.
4. **BinaryPanel Dashboard**: Fast, zero-dependency SPA connecting it all via APIs.

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
- **Rootless Compatibility**: Docker configurations support native Unix sockets and rootless setups where user groups possess appropriate permissions.

## 📜 Licensing and Dependencies

BinaryPanel is entirely **Free and Open-Source** under the MIT license. 
Rest assured, there are **no enterprise paywalls**. Every dependency we use is specifically chosen because it permits commercial and non-commercial redistribution with no strings attached:
- **Caddy**: Apache 2.0 License
- **FileBrowser**: Apache 2.0 License
- **Portainer CE**: zlib License

Enjoy your modern, native, and fully composable control panel!
