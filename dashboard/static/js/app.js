/* ═══════════════════════════════════════════════════════════════
   BinaryPanel — Core Application Logic
   ═══════════════════════════════════════════════════════════════ */

const BinaryPanel = {
    currentSection: 'dashboard',
    refreshInterval: null,
    serviceLinks: {},

    // ── Initialization ──
    async init() {
        this.bindEvents();
        const session = await this.checkSession();
        if (session.authenticated) {
            this.showApp(session.username);
        } else {
            this.showLogin();
        }
    },

    // ── Event Binding ──
    bindEvents() {
        // Login form
        document.getElementById('login-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.login();
        });

        // Logout
        document.getElementById('logout-btn').addEventListener('click', () => this.logout());

        // Navigation
        document.querySelectorAll('.nav-item[data-section]').forEach(item => {
            item.addEventListener('click', (e) => {
                e.preventDefault();
                this.navigate(item.dataset.section);
            });
        });

        // Mobile menu
        const mobileBtn = document.getElementById('mobile-menu-btn');
        if (mobileBtn) {
            mobileBtn.addEventListener('click', () => this.toggleSidebar());
        }

        window.addEventListener('hashchange', () => {
            const hash = location.hash.slice(1) || 'dashboard';
            if (['dashboard', 'domains', 'containers', 'system', 'settings', 'apps'].includes(hash)) {
                this.navigate(hash, false);
            }
        });

        // Close modals on overlay click
        document.querySelectorAll('.modal-overlay').forEach(overlay => {
            overlay.addEventListener('click', (e) => {
                if (e.target === overlay) {
                    overlay.style.display = 'none';
                }
            });
        });

        // Quick actions
        document.getElementById('action-open-files').addEventListener('click', () => {
            if (this.serviceLinks.filebrowser) {
                window.open(this.serviceLinks.filebrowser, '_blank');
            }
        });
        document.getElementById('action-open-portainer').addEventListener('click', () => {
            if (this.serviceLinks.portainer) {
                window.open(this.serviceLinks.portainer, '_blank');
            }
        });

        // 1-Click App Deployment
        const deployForm = document.getElementById('app-deploy-form');
        if (deployForm) {
            deployForm.addEventListener('submit', async (e) => {
                e.preventDefault();
                const port = document.getElementById('app-deploy-port').value;
                const btn = document.getElementById('app-deploy-btn');
                const status = document.getElementById('app-deploy-status');
                
                btn.disabled = true;
                btn.querySelector('.btn-text').textContent = 'Deploying BinaryCMS (Please wait 60s)...';
                btn.querySelector('.btn-loader').style.display = 'inline-block';
                status.textContent = "Pulling repository and building image securely. Do not close this window...";
                status.style.color = "#3b82f6";
                
                try {
                    const res = await this.api('/api/apps/deploy/binarycms', {
                        method: 'POST',
                        body: JSON.stringify({ port })
                    });
                    
                    status.textContent = "⏳ Compiling native image and pulling layers... (This will take up to 60 seconds)";
                    status.style.color = "#3b82f6";
                    
                    let isRunning = false;
                    if (res.container) {
                        // Poll API to check if container is running
                        for (let i = 0; i < 40; i++) {
                            await new Promise(r => setTimeout(r, 3000));
                            try {
                                const containersRes = await this.api('/api/containers');
                                const containers = containersRes.containers || [];
                                const c = containers.find(x => x.name === res.container);
                                if (c && c.state === 'running') {
                                    isRunning = true;
                                    break;
                                }
                            } catch (e) {
                                // Ignore polling errors
                            }
                        }
                    }

                    if (isRunning) {
                        status.textContent = "✅ BinaryCMS successfully deployed and fully operational on Port " + port;
                        status.style.color = "#10b981";
                    } else {
                        status.textContent = "⚠️ Deployment initiated, but timed out waiting for 'running' state. Please check the Containers tab.";
                        status.style.color = "#f59e0b";
                    }
                } catch (err) {
                    status.textContent = "❌ " + err.message;
                    status.style.color = "#ef4444";
                } finally {
                    btn.disabled = false;
                    btn.querySelector('.btn-text').textContent = '🚀 1-Click Deploy';
                    btn.querySelector('.btn-loader').style.display = 'none';
                }
            });
        }
        // UI Login Credentials Override
        const authForm = document.getElementById('auth-settings-form');
        if (authForm) {
            authForm.addEventListener('submit', async (e) => {
                e.preventDefault();
                const username = document.getElementById('auth-username').value;
                const password = document.getElementById('auth-password').value;
                const status = document.getElementById('auth-save-status');
                
                try {
                    await this.api('/api/settings/auth', {
                        method: 'POST',
                        body: JSON.stringify({ username, password })
                    });
                    status.textContent = "✅ Credentials rewritten into encrypted memory securely!";
                    status.style.color = "#10b981";
                    setTimeout(() => status.textContent = '', 4000);
                } catch(err) {
                    status.textContent = "❌ " + err.message;
                    status.style.color = "#ef4444";
                }
            });
        }

        // Master Background Self-Updater
        const systemUpdateBtn = document.getElementById('system-update-btn');
        if (systemUpdateBtn) {
            systemUpdateBtn.addEventListener('click', async () => {
                const status = document.getElementById('system-update-status');
                const loader = systemUpdateBtn.querySelector('.btn-loader');
                
                if (!await this.confirm("Deploy Infrastructure Update", "Are you sure you want to run the root container orchestrator? The interface will safely detach while compiling.")) return;

                systemUpdateBtn.disabled = true;
                loader.style.display = 'inline-block';
                status.textContent = "Pulling global changes & tearing down UI daemon securely...";
                status.style.color = "#3b82f6";
                
                try {
                    await this.api('/api/system/update', { method: 'POST' });
                    status.textContent = "Rebuilding daemon stack... interface disconnected safely. Please wait.";
                    
                    // Polling loop waiting for the container daemon to structurally restart
                    const pollInterval = setInterval(async () => {
                        try {
                            const ping = await fetch('/api/system/stats');
                            if (ping.ok) {
                                clearInterval(pollInterval);
                                window.location.reload();
                            }
                        } catch(e) {}
                    }, 5000);
                    
                } catch(err) {
                    status.textContent = "❌ " + err.message;
                    status.style.color = "#ef4444";
                    systemUpdateBtn.disabled = false;
                    loader.style.display = 'none';
                }
            });
        }

        // ── System Stack Reboot ──
        const rebootBtn = document.getElementById('system-reboot-btn');
        if (rebootBtn) {
            rebootBtn.addEventListener('click', async () => {
                const status = document.getElementById('system-reboot-status');
                const loader = rebootBtn.querySelector('.btn-loader');

                if (!await this.confirm("Reboot All Services", "This will restart all BinaryPanel containers (Caddy, Dashboard, FileBrowser). The dashboard will temporarily disconnect. Continue?")) return;

                rebootBtn.disabled = true;
                loader.style.display = 'inline-block';
                status.textContent = "Rebooting all services...";
                status.style.color = "#3b82f6";

                try {
                    await this.api('/api/system/reboot-stack', { method: 'POST' });
                    status.textContent = "Services restarting... auto-reconnecting in ~15 seconds.";

                    // Auto-reconnect polling
                    const pollInterval = setInterval(async () => {
                        try {
                            const ping = await fetch('/api/system/stats');
                            if (ping.ok) {
                                clearInterval(pollInterval);
                                status.textContent = "✅ All services back online!";
                                status.style.color = "#10b981";
                                rebootBtn.disabled = false;
                                loader.style.display = 'none';
                                // Reload data
                                this.loadSectionData(this.currentSection);
                            }
                        } catch(e) {}
                    }, 3000);

                } catch(err) {
                    status.textContent = "❌ " + err.message;
                    status.style.color = "#ef4444";
                    rebootBtn.disabled = false;
                    loader.style.display = 'none';
                }
            });
        }

        // ── System Error Log Viewer ──
        const logsBtn = document.getElementById('system-logs-btn');
        if (logsBtn) {
            logsBtn.addEventListener('click', () => this.openSystemLog());
        }

        const syslogClose = document.getElementById('syslog-modal-close');
        if (syslogClose) {
            syslogClose.addEventListener('click', () => {
                document.getElementById('syslog-modal').style.display = 'none';
            });
        }

        const syslogRefresh = document.getElementById('syslog-refresh');
        if (syslogRefresh) {
            syslogRefresh.addEventListener('click', () => this.loadSystemLog());
        }

        // Log level filter buttons
        document.querySelectorAll('.syslog-filter').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.syslog-filter').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                this.loadSystemLog(btn.dataset.level);
            });
        });
    },

    // ── System Log Methods ──
    async openSystemLog() {
        document.getElementById('syslog-modal').style.display = 'flex';
        this.loadSystemLog();
    },

    async loadSystemLog(level = '') {
        const content = document.getElementById('syslog-content');
        content.textContent = 'Loading logs...';

        try {
            let url = '/api/system/logs?limit=100';
            if (level) url += '&level=' + level;
            const data = await this.api(url);

            if (!data || !data.entries || data.entries.length === 0) {
                content.innerHTML = '<span style="color: #64748b;">No log entries found. The system is running cleanly.</span>';
                return;
            }

            const levelColors = {
                'ERROR': '#ef4444',
                'WARN': '#f59e0b',
                'INFO': '#3b82f6'
            };

            let html = '';
            for (const entry of data.entries) {
                const color = levelColors[entry.level] || '#94a3b8';
                const ts = entry.timestamp.replace('T', ' ').replace('Z', ' UTC');
                html += `<span style="color: #64748b;">${this.escapeHtml(ts)}</span> `;
                html += `<span style="color: ${color}; font-weight: 600;">[${entry.level}]</span> `;
                html += `<span style="color: #8b5cf6;">[${this.escapeHtml(entry.component)}]</span> `;
                html += this.escapeHtml(entry.message);
                if (entry.context) {
                    html += `\n  <span style="color: #fbbf24;">↳ ${this.escapeHtml(entry.context)}</span>`;
                }
                html += '\n';
            }
            content.innerHTML = html;
        } catch (err) {
            content.textContent = 'Failed to load logs: ' + err.message;
        }
    },

    // ── Auth ──
    async checkSession() {
        try {
            const resp = await fetch('/api/auth/session');
            return await resp.json();
        } catch {
            return { authenticated: false };
        }
    },

    async login() {
        const btn = document.getElementById('login-btn');
        const btnText = btn.querySelector('.btn-text');
        const btnLoader = btn.querySelector('.btn-loader');
        const errorEl = document.getElementById('login-error');

        btnText.style.display = 'none';
        btnLoader.style.display = 'inline-block';
        errorEl.style.display = 'none';

        try {
            const resp = await fetch('/api/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    username: document.getElementById('login-username').value,
                    password: document.getElementById('login-password').value,
                }),
            });

            const data = await resp.json();
            if (resp.ok && data.success) {
                this.showApp(data.username);
            } else {
                errorEl.textContent = data.error || 'Invalid credentials';
                errorEl.style.display = 'block';
            }
        } catch (err) {
            errorEl.textContent = 'Connection error. Is the server running?';
            errorEl.style.display = 'block';
        } finally {
            btnText.style.display = 'inline';
            btnLoader.style.display = 'none';
        }
    },

    async logout() {
        await fetch('/api/auth/logout', { method: 'POST' });
        this.stopRefresh();
        this.showLogin();
    },

    // ── Screen Switching ──
    showLogin() {
        document.getElementById('login-screen').style.display = 'flex';
        document.getElementById('app').style.display = 'none';
    },

    showApp(username) {
        document.getElementById('login-screen').style.display = 'none';
        document.getElementById('app').style.display = 'flex';

        // Set user info
        document.getElementById('user-name').textContent = username;
        document.getElementById('user-avatar').textContent = username.charAt(0).toUpperCase();

        // Load data
        this.loadServiceLinks();
        this.navigate(location.hash.slice(1) || 'dashboard', false);
        this.startRefresh();
    },

    // ── Navigation ──
    navigate(section, updateHash = true) {
        if (updateHash) {
            location.hash = section;
        }

        this.currentSection = section;

        // Update nav items
        document.querySelectorAll('.nav-item[data-section]').forEach(item => {
            item.classList.toggle('active', item.dataset.section === section);
        });

        // Show section
        document.querySelectorAll('.section').forEach(sec => {
            sec.classList.remove('active');
        });
        const target = document.getElementById(`section-${section}`);
        if (target) {
            target.classList.add('active');
        }

        // Load section data
        this.loadSectionData(section);

        // Close mobile sidebar
        document.querySelector('.sidebar').classList.remove('open');
    },

    loadSectionData(section) {
        switch (section) {
            case 'dashboard':
                SystemModule.loadStats();
                ContainersModule.loadContainers();
                DomainsModule.loadDomains();
                break;
            case 'domains':
                DomainsModule.loadDomains();
                break;
            case 'containers':
                ContainersModule.loadContainers();
                break;
            case 'system':
                SystemModule.loadStats();
                break;
            case 'settings':
                SecurityModule.loadBlockedIPs();
                break;
        }
    },

    // ── Service Links ──
    async loadServiceLinks() {
        try {
            const resp = await fetch('/api/links');
            if (resp.ok) {
                this.serviceLinks = await resp.json();

                // Update nav links
                const fbLink = document.getElementById('nav-filebrowser');
                const ptLink = document.getElementById('nav-portainer');
                if (this.serviceLinks.filebrowser) {
                    fbLink.href = this.serviceLinks.filebrowser;
                }
                if (this.serviceLinks.portainer) {
                    ptLink.href = this.serviceLinks.portainer;
                }
            }
        } catch {
            // Silent fail — links stay as #
        }
    },

    // ── Auto-Refresh ──
    startRefresh() {
        this.stopRefresh();
        this.refreshInterval = setInterval(() => {
            if (this.currentSection === 'dashboard' || this.currentSection === 'system') {
                SystemModule.loadStats();
            }
        }, 5000);
    },

    stopRefresh() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
            this.refreshInterval = null;
        }
    },

    // ── Sidebar Toggle (Mobile) ──
    toggleSidebar() {
        document.querySelector('.sidebar').classList.toggle('open');
    },

    // ── Toast Notifications ──
    toast(message, type = 'info') {
        const container = document.getElementById('toast-container');
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;
        toast.innerHTML = `
            <span>${this.escapeHtml(message)}</span>
        `;
        container.appendChild(toast);

        setTimeout(() => {
            toast.classList.add('removing');
            setTimeout(() => toast.remove(), 300);
        }, 4000);
    },

    // ── Confirm Dialog ──
    confirm(title, message) {
        return new Promise((resolve) => {
            document.getElementById('confirm-title').textContent = title;
            document.getElementById('confirm-message').textContent = message;
            document.getElementById('confirm-modal').style.display = 'flex';

            const ok = document.getElementById('confirm-ok');
            const cancel = document.getElementById('confirm-cancel');
            const close = document.getElementById('confirm-close');

            const cleanup = () => {
                document.getElementById('confirm-modal').style.display = 'none';
                ok.replaceWith(ok.cloneNode(true));
                cancel.replaceWith(cancel.cloneNode(true));
                close.replaceWith(close.cloneNode(true));
            };

            ok.addEventListener('click', () => { cleanup(); resolve(true); }, { once: true });
            cancel.addEventListener('click', () => { cleanup(); resolve(false); }, { once: true });
            close.addEventListener('click', () => { cleanup(); resolve(false); }, { once: true });
        });
    },

    // ── Utility ──
    escapeHtml(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    },

    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
    },

    async api(url, options = {}) {
        return this.apiRequest(url, options);
    },

    async apiRequest(url, options = {}) {
        try {
            const resp = await fetch(url, {
                ...options,
                headers: {
                    'Content-Type': 'application/json',
                    ...options.headers,
                },
            });
            const data = await resp.json();
            if (!resp.ok) {
                throw new Error(data.error || `HTTP ${resp.status}`);
            }
            return data;
        } catch (err) {
            if (err.message.includes('unauthorized') || err.message.includes('session expired')) {
                this.showLogin();
                return null;
            }
            throw err;
        }
    }
};

// ── Boot ──
document.addEventListener('DOMContentLoaded', () => BinaryPanel.init());
