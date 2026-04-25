/* ═══════════════════════════════════════════════════════════════
   BinaryPanel — Domain Security Module
   ═══════════════════════════════════════════════════════════════ */

const SecurityModule = {
    currentDomain: null,
    securityStatuses: {},

    // ── Load security statuses for all domains ──
    async loadStatuses() {
        try {
            const data = await BinaryPanel.apiRequest('/api/security/domains');
            if (!data || !data.domains) return;
            this.securityStatuses = {};
            for (const d of data.domains) {
                this.securityStatuses[d.domain] = d;
            }
        } catch (err) {
            console.error('Failed to load security statuses:', err);
        }
    },

    // Get security status for a specific domain
    getStatus(domain) {
        return this.securityStatuses[domain] || {
            domain: domain,
            security_score: 0,
            headers_enabled: false,
            ip_restricted: false
        };
    },

    // ── Open security config modal ──
    async openConfig(domainName) {
        this.currentDomain = domainName;
        const modal = document.getElementById('security-modal');
        modal.style.display = 'flex';

        document.getElementById('security-domain-name').textContent = domainName;

        // Reset form
        this.resetForm();

        // Load current config
        try {
            const data = await BinaryPanel.apiRequest(`/api/security/domain?name=${encodeURIComponent(domainName)}`);
            if (data && data.config) {
                this.populateForm(data.config);
                this.renderScore(data.score);
            }
        } catch (err) {
            BinaryPanel.toast('Failed to load security config: ' + err.message, 'error');
        }
    },

    closeModal() {
        document.getElementById('security-modal').style.display = 'none';
        this.currentDomain = null;
    },

    resetForm() {
        document.getElementById('sec-hsts').checked = false;
        document.getElementById('sec-hsts-maxage').value = '31536000';
        document.getElementById('sec-hsts-subdomains').checked = false;
        document.getElementById('sec-hsts-preload').checked = false;
        document.getElementById('sec-xframe').value = '';
        document.getElementById('sec-xcontent').checked = false;
        document.getElementById('sec-referrer').value = '';
        document.getElementById('sec-permissions').value = '';
        document.getElementById('sec-csp-enabled').checked = false;
        document.getElementById('sec-csp-value').value = '';
        document.getElementById('sec-ip-whitelist').value = '';
        document.getElementById('sec-ip-blacklist').value = '';
        this.renderScore(0);
    },

    populateForm(cfg) {
        document.getElementById('sec-hsts').checked = cfg.hsts_enabled || false;
        document.getElementById('sec-hsts-maxage').value = cfg.hsts_max_age || 31536000;
        document.getElementById('sec-hsts-subdomains').checked = cfg.hsts_subdomains || false;
        document.getElementById('sec-hsts-preload').checked = cfg.hsts_preload || false;
        document.getElementById('sec-xframe').value = cfg.x_frame_options || '';
        document.getElementById('sec-xcontent').checked = cfg.x_content_type_opts || false;
        document.getElementById('sec-referrer').value = cfg.referrer_policy || '';
        document.getElementById('sec-permissions').value = cfg.permissions_policy || '';
        document.getElementById('sec-csp-enabled').checked = cfg.csp_enabled || false;
        document.getElementById('sec-csp-value').value = cfg.csp_value || '';
        document.getElementById('sec-ip-whitelist').value = (cfg.ip_whitelist || []).join('\n');
        document.getElementById('sec-ip-blacklist').value = (cfg.ip_blacklist || []).join('\n');

        // Toggle HSTS options visibility
        this.toggleHSTSOptions();
        this.toggleCSPOptions();
    },

    toggleHSTSOptions() {
        const enabled = document.getElementById('sec-hsts').checked;
        document.getElementById('hsts-options').style.display = enabled ? 'block' : 'none';
    },

    toggleCSPOptions() {
        const enabled = document.getElementById('sec-csp-enabled').checked;
        document.getElementById('csp-options').style.display = enabled ? 'block' : 'none';
    },

    // ── Build config from form ──
    buildConfig() {
        const ipWhitelist = document.getElementById('sec-ip-whitelist').value
            .split('\n').map(s => s.trim()).filter(s => s.length > 0);
        const ipBlacklist = document.getElementById('sec-ip-blacklist').value
            .split('\n').map(s => s.trim()).filter(s => s.length > 0);

        return {
            domain: this.currentDomain,
            hsts_enabled: document.getElementById('sec-hsts').checked,
            hsts_max_age: parseInt(document.getElementById('sec-hsts-maxage').value) || 31536000,
            hsts_subdomains: document.getElementById('sec-hsts-subdomains').checked,
            hsts_preload: document.getElementById('sec-hsts-preload').checked,
            x_frame_options: document.getElementById('sec-xframe').value,
            x_content_type_opts: document.getElementById('sec-xcontent').checked,
            referrer_policy: document.getElementById('sec-referrer').value,
            permissions_policy: document.getElementById('sec-permissions').value,
            csp_enabled: document.getElementById('sec-csp-enabled').checked,
            csp_value: document.getElementById('sec-csp-value').value,
            ip_whitelist: ipWhitelist,
            ip_blacklist: ipBlacklist
        };
    },

    // ── Save config ──
    async saveConfig() {
        if (!this.currentDomain) return;

        const cfg = this.buildConfig();
        const saveBtn = document.getElementById('security-save-btn');

        try {
            saveBtn.disabled = true;
            saveBtn.textContent = 'Applying...';

            const result = await BinaryPanel.apiRequest('/api/security/domain', {
                method: 'PUT',
                body: JSON.stringify(cfg)
            });

            if (result && result.score !== undefined) {
                this.renderScore(result.score);
            }

            BinaryPanel.toast(`Security config applied for ${this.currentDomain}`, 'success');

            // Refresh the domain list to update badges
            await this.loadStatuses();
            DomainsModule.loadDomains();

        } catch (err) {
            BinaryPanel.toast('Failed to apply security config: ' + err.message, 'error');
        } finally {
            saveBtn.disabled = false;
            saveBtn.textContent = '🛡️ Apply Security Config';
        }
    },

    // ── Quick Presets ──
    applyPreset(level) {
        switch (level) {
            case 'strict':
                document.getElementById('sec-hsts').checked = true;
                document.getElementById('sec-hsts-maxage').value = '31536000';
                document.getElementById('sec-hsts-subdomains').checked = true;
                document.getElementById('sec-hsts-preload').checked = true;
                document.getElementById('sec-xframe').value = 'DENY';
                document.getElementById('sec-xcontent').checked = true;
                document.getElementById('sec-referrer').value = 'strict-origin';
                document.getElementById('sec-permissions').value = 'camera=(), microphone=(), geolocation=()';
                document.getElementById('sec-csp-enabled').checked = true;
                document.getElementById('sec-csp-value').value = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'";
                break;
            case 'balanced':
                document.getElementById('sec-hsts').checked = true;
                document.getElementById('sec-hsts-maxage').value = '15768000';
                document.getElementById('sec-hsts-subdomains').checked = false;
                document.getElementById('sec-hsts-preload').checked = false;
                document.getElementById('sec-xframe').value = 'SAMEORIGIN';
                document.getElementById('sec-xcontent').checked = true;
                document.getElementById('sec-referrer').value = 'strict-origin-when-cross-origin';
                document.getElementById('sec-permissions').value = '';
                document.getElementById('sec-csp-enabled').checked = false;
                document.getElementById('sec-csp-value').value = '';
                break;
            case 'minimal':
                document.getElementById('sec-hsts').checked = true;
                document.getElementById('sec-hsts-maxage').value = '2592000';
                document.getElementById('sec-hsts-subdomains').checked = false;
                document.getElementById('sec-hsts-preload').checked = false;
                document.getElementById('sec-xframe').value = '';
                document.getElementById('sec-xcontent').checked = true;
                document.getElementById('sec-referrer').value = '';
                document.getElementById('sec-permissions').value = '';
                document.getElementById('sec-csp-enabled').checked = false;
                document.getElementById('sec-csp-value').value = '';
                break;
        }
        this.toggleHSTSOptions();
        this.toggleCSPOptions();

        // Highlight active preset
        document.querySelectorAll('.preset-btn').forEach(b => b.classList.remove('active'));
        document.querySelector(`.preset-btn[data-preset="${level}"]`)?.classList.add('active');
    },

    // ── Render score gauge ──
    renderScore(score) {
        const el = document.getElementById('security-score-value');
        const circle = document.getElementById('security-score-fill');
        const label = document.getElementById('security-score-label');

        if (el) el.textContent = score;

        if (circle) {
            const circumference = 2 * Math.PI * 40;
            const offset = circumference - (score / 100) * circumference;
            circle.style.strokeDasharray = circumference;
            circle.style.strokeDashoffset = offset;

            // Color based on score
            if (score >= 80) {
                circle.style.stroke = 'var(--green)';
            } else if (score >= 40) {
                circle.style.stroke = 'var(--amber)';
            } else {
                circle.style.stroke = 'var(--red)';
            }
        }

        if (label) {
            if (score >= 80) label.textContent = 'Well Protected';
            else if (score >= 40) label.textContent = 'Partial Protection';
            else label.textContent = 'Unprotected';
        }
    },

    // ── Global IP Blocklist Management ──
    async loadBlockedIPs() {
        try {
            const data = await BinaryPanel.apiRequest('/api/security/ips');
            if (!data) return;

            const list = document.getElementById('blocked-ips-list');
            if (!list) return;

            const ips = data.blocked_ips || [];
            if (ips.length === 0) {
                list.innerHTML = '<p style="color: var(--text-muted); font-size: 0.875rem;">No IPs are currently blocked.</p>';
                return;
            }

            list.innerHTML = ips.map(ip => `
                <div class="ip-entry">
                    <div class="ip-info">
                        <code>${BinaryPanel.escapeHtml(ip.ip)}</code>
                        <span class="ip-reason">${BinaryPanel.escapeHtml(ip.reason || 'No reason')}</span>
                        <span class="ip-date">${ip.blocked_at ? new Date(ip.blocked_at).toLocaleDateString() : ''}</span>
                    </div>
                    <button class="btn btn-sm btn-ghost" style="color: var(--red);" onclick="SecurityModule.unblockIP('${BinaryPanel.escapeHtml(ip.ip)}')">Unblock</button>
                </div>
            `).join('');
        } catch (err) {
            console.error('Failed to load blocked IPs:', err);
        }
    },

    async blockIP() {
        const input = document.getElementById('block-ip-input');
        const reason = document.getElementById('block-ip-reason');
        const ip = input.value.trim();

        if (!ip) {
            BinaryPanel.toast('Please enter an IP address', 'error');
            return;
        }

        try {
            await BinaryPanel.apiRequest('/api/security/ips/block', {
                method: 'POST',
                body: JSON.stringify({ ip: ip, reason: reason.value.trim() })
            });
            BinaryPanel.toast(`IP ${ip} blocked successfully`, 'success');
            input.value = '';
            reason.value = '';
            this.loadBlockedIPs();
        } catch (err) {
            BinaryPanel.toast('Failed to block IP: ' + err.message, 'error');
        }
    },

    async unblockIP(ip) {
        if (!await BinaryPanel.confirm('Unblock IP', `Remove ${ip} from the global blocklist?`)) return;

        try {
            await BinaryPanel.apiRequest('/api/security/ips/unblock', {
                method: 'DELETE',
                body: JSON.stringify({ ip: ip })
            });
            BinaryPanel.toast(`IP ${ip} unblocked`, 'success');
            this.loadBlockedIPs();
        } catch (err) {
            BinaryPanel.toast('Failed to unblock IP: ' + err.message, 'error');
        }
    },

    // ── Initialize ──
    init() {
        // Close modal
        const closeBtn = document.getElementById('security-modal-close');
        if (closeBtn) {
            closeBtn.addEventListener('click', () => this.closeModal());
        }

        // Save button
        const saveBtn = document.getElementById('security-save-btn');
        if (saveBtn) {
            saveBtn.addEventListener('click', () => this.saveConfig());
        }

        // HSTS toggle
        const hstsToggle = document.getElementById('sec-hsts');
        if (hstsToggle) {
            hstsToggle.addEventListener('change', () => this.toggleHSTSOptions());
        }

        // CSP toggle
        const cspToggle = document.getElementById('sec-csp-enabled');
        if (cspToggle) {
            cspToggle.addEventListener('change', () => this.toggleCSPOptions());
        }

        // Preset buttons
        document.querySelectorAll('.preset-btn').forEach(btn => {
            btn.addEventListener('click', () => this.applyPreset(btn.dataset.preset));
        });

        // Block IP button
        const blockBtn = document.getElementById('block-ip-btn');
        if (blockBtn) {
            blockBtn.addEventListener('click', () => this.blockIP());
        }
    }
};

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    SecurityModule.init();
});
