# BinaryPanel Ecosystem: Complete User Guide

Welcome to the BinaryPanel Ecosystem! This is your definitive manual for operating your new lightweight, zero-bloat server infrastructure. BinaryPanel unifies powerful Go-native services into a single UI, giving you everything you need to host applications, proxy traffic, and manage files.

---

## 0. Prerequisite: Domain DNS Configuration (Required)

Before you add any domains to BinaryPanel, **you must point your domain to this server's IP address** via your Domain Registrar (e.g., Namecheap, GoDaddy, Cloudflare). If you do not do this first, BinaryPanel (Caddy) will fail to generate your automatic Let's Encrypt SSL certificates.

### How to set up your DNS:
1. Log into your Domain Registrar's dashboard.
2. Navigate to **DNS Management** or **Advanced DNS** for your domain.
3. Add an **A Record**:
   - **Host / Name**: `@` (which represents the root domain, e.g. `yourdomain.com`)
   - **Value / Target**: Paste the `Public IP Address` of this Ubuntu server.
   - **TTL**: Automatic or 5 min.
4. *(Optional)* Add a second **A Record** for subdomains:
   - **Host / Name**: `www` (or `*` to catch all subdomains)
   - **Value / Target**: Paste your Server IP.

*Note: DNS changes can occasionally take 5-30 minutes to propagate across the globe. Wait a few moments before trying to add the domain inside BinaryPanel.*

---

## 1. Initial Setup & First Login

When you execute the `install.sh` command, your ecosystem spins up on the following ports:

*   **BinaryPanel Dashboard**: `http://<your-ip>:9000`
*   **FileBrowser**: `http://<your-ip>:8090`
*   **Portainer**: `https://<your-ip>:9443`

### The First Things You MUST Do:
1.  **Log into BinaryPanel** (`admin` / `admin`) and familiarize yourself with the interface.
2.  **Log into FileBrowser** (`admin` / `Admin123456!`). Instantly navigate to _Settings > Profile Management_ and change your password.
3.  **Log into Portainer**. Because this is the first time you are booting it, Portainer will ask you to create a brand new Administrator password.

---

## 2. Navigating the BinaryPanel Dashboard

### System Overview
The main screen provides up-to-the-second readings on your Server load, RAM usage, Uptime, and CPU stress by securely querying your system's `loadavg` and `/proc` files. 

### Domain Management (The Reverse Proxy)
Our domain manager talks directly to the Caddy API. You can add domains to your server and Caddy will **automatically provision SSL certificates (Let's Encrypt)** for them. 

When you click "Add Domain", you have two options:
1.  **Reverse Proxy**: If you have a Docker container running an app on Port `3000`, add your domain (e.g., `app.mywebsite.com`) and set the upstream to `localhost:3000`. Caddy instantly routes all secure traffic from your domain to that port!
2.  **File Server**: If you just want to host a static website (HTML/CSS), select File Server, and point the upstream to a local folder in your `storage` directory. 

---

## 3. Email Architecture (Sending vs. Receiving)

Modern server architecture separates *sending automated emails* from *hosting user inboxes*. 

### A) Receiving Email (Inboxes via Google Workspace / Zoho)
To read, reply, and host real mailboxes (e.g., `you@yourdomain.com`), you should use a dedicated secure host like **Google Workspace**, **Microsoft 365**, or **Zoho Mail** (which offers free tiers).

**How to set up Incoming Mail:**
1. Create an account on your preferred platform (e.g., Google Workspace) and type in your domain name.
2. They will provide you with **MX Records** (Mail Exchanger records).
3. Go back to your **Domain Registrar's DNS Settings** (where you pointed your A record in Step 0).
4. Create the `MX` records they provide. *(Example for Google: `ASPMX.L.GOOGLE.COM` on Priority 1).*
5. You can now log into Gmail/Zoho and read all incoming emails securely, separate from your web infrastructure.

### B) Sending Application Mail (BinaryPanel Relay Settings)
For your website to send purely automated emails (like "Password Resets" or "Welcome Emails"), BinaryPanel features a built-in SMTP relay integration.
1. Navigate to **BinaryPanel Dashboard > Settings**.
2. Select an outbound provider from the dropdown (e.g., **Resend**, **Amazon SES**, **Mailgun**).
3. Enter your API Key as the password, and test the connection. This setting persists securely across your entire app infrastructure in `/data/settings.json`.

---

## 4. Storage Management: FileBrowser

FileBrowser acts as your modern "Web FTP." Everything inside FileBrowser maps exactly to the `/opt/binarypanel/storage` folder on your server machine.

*   **Uploading Code**: Just drag and drop your web application files.
*   **Editing Code**: Clicking on any text or code file opens a built-in VS Code-style syntax editor in your browser.

**Pro-Tip**: If you create a folder named `/srv/my_react_app` in FileBrowser and upload your static build files there, you can jump over to BinaryPanel and create a **File Server Domain** pointing directly to `/srv/my_react_app` to make it instantly live!

---

## 5. App Deployment: Portainer

Portainer is where the heavy lifting happens. It visualizing the raw Docker Socket API.

### How to Host a Complex Project (e.g. A Node.js App or Database):
1.  Click into **Local Environment**.
2.  Navigate to **Containers** -> **Add Container**.
3.  Type in the Docker image you want (e.g. `mysql:8.0` or `node:18`).
4.  **Network Ports**: Map your container's internal port to an unused host port (For example, map Host: `8080` to Container: `80`). 
5.  Click **Deploy the container**.
6.  *Final Step*: Jump back to the **BinaryPanel Dashboard -> Domains**, and proxy your domain to `localhost:8080`. Your app is now live AND secured with SSL!

---

## 6. Security & Maintenance

*   **Zero-Downtime Reboots**: Caddy reads configuration straight from memory. Even if you reboot BinaryPanel or Portainer, Caddy will continue proxying traffic seamlessly.
*   **Updates**: Because everything operates via standard Docker compose structures, updating the system is as simple as running:
    ```bash
    cd /opt/binarypanel && git pull && docker compose up -d --build
    ```

**Enjoy your unified, composable infrastructure!**
