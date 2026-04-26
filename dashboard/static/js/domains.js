/* ═══════════════════════════════════════════════════════════════
   BinaryPanel — Domain Management Module
   ═══════════════════════════════════════════════════════════════ */

const DomainsModule = {
    domains: [],

    async loadDomains() {
        try {
            const data = await BinaryPanel.apiRequest('/api/domains');
            if (!data) return;

            this.domains = data.domains || [];

            // Also load security statuses
            await SecurityModule.loadStatuses();

            this.renderDomains();
            this.updateDashboardStat();
        } catch (err) {
            console.error('Failed to load domains:', err);
        }
    },

    renderDomains() {
        const tbody = document.getElementById('domains-tbody');
        const empty = document.getElementById('domains-empty');
        const tableWrap = document.getElementById('domains-table-wrap');

        if (!this.domains.length) {
            empty.style.display = 'block';
            tableWrap.style.display = 'none';
            return;
        }

        empty.style.display = 'none';
        tableWrap.style.display = 'block';

        tbody.innerHTML = this.domains.map((d, i) => {
            const domainName = d.domains ? d.domains[0] : '';
            const secStatus = SecurityModule.getStatus(domainName);
            const score = secStatus.security_score || 0;
            let scoreBadgeClass = 'badge-red';
            let scoreIcon = '🔴';
            if (score >= 80) { scoreBadgeClass = 'badge-green'; scoreIcon = '🟢'; }
            else if (score >= 40) { scoreBadgeClass = 'badge-amber'; scoreIcon = '🟡'; }

            return `
            <tr>
                <td>
                    <span class="domain-name">${BinaryPanel.escapeHtml(d.domains ? d.domains.join(', ') : '—')}</span>
                </td>
                <td>
                    <code style="font-family:var(--font-mono);font-size:13px;color:var(--text-secondary)">${BinaryPanel.escapeHtml(d.upstream || '—')}</code>
                </td>
                <td>
                    <span class="badge ${d.type === 'reverse_proxy' ? 'badge-blue' : 'badge-amber'}">
                        ${d.type === 'reverse_proxy' ? '⇄ Proxy' : '📁 Files'}
                    </span>
                </td>
                <td>
                    <span class="ssl-active">
                        <span class="ssl-dot"></span>Auto SSL
                    </span>
                </td>
                <td>
                    <span class="badge ${scoreBadgeClass}" title="Security Score: ${score}/100">
                        ${scoreIcon} ${score}/100
                    </span>
                </td>
                <td>
                    <div class="table-actions">
                        <button class="btn-icon" style="color: var(--purple)" title="Security Settings" onclick="SecurityModule.openConfig('${BinaryPanel.escapeHtml(domainName)}')">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
                        </button>
                        ${d.type === 'reverse_proxy' && d.upstream ? `
                        <button class="btn-icon" style="color: var(--blue)" title="Restart Server" onclick="DomainsModule.triggerRestart(${d.id}, '${BinaryPanel.escapeHtml(domainName)}')">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
                        </button>
                        <button class="btn-icon" style="color: var(--blue)" title="Download Backup" onclick="DomainsModule.triggerBackup(${d.id}, '${BinaryPanel.escapeHtml(domainName)}')">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                        </button>
                        <button class="btn-icon" style="color: var(--blue)" title="Restore Backup" onclick="DomainsModule.triggerRestore(${d.id}, '${BinaryPanel.escapeHtml(domainName)}')">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>
                        </button>
                        ` : ''}
                        <button class="btn-icon" title="Edit" onclick="DomainsModule.editDomain(${d.id})">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                        </button>
                        <button class="btn-icon" title="Delete" onclick="DomainsModule.deleteDomain(${d.id}, '${BinaryPanel.escapeHtml(domainName)}')">
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--red)" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
                        </button>
                    </div>
                </td>
            </tr>
        `}).join('');
    },

    updateDashboardStat() {
        const el = document.getElementById('stat-domains');
        if (el) el.textContent = this.domains.length;
    },

    // ── Add Domain Modal ──
    init() {
        document.getElementById('add-domain-btn').addEventListener('click', () => this.openModal());
        document.getElementById('domain-modal-close').addEventListener('click', () => this.closeModal());
        document.getElementById('domain-cancel-btn').addEventListener('click', () => this.closeModal());
        document.getElementById('domain-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.saveDomain();
        });

        // Update label on type change
        document.getElementById('domain-type').addEventListener('change', (e) => {
            const label = document.getElementById('domain-upstream-label');
            const hint = document.getElementById('domain-upstream-hint');
            const input = document.getElementById('domain-upstream');
            if (e.target.value === 'file_server') {
                label.textContent = 'Root Directory';
                hint.textContent = 'The directory to serve files from';
                input.placeholder = '/var/www/html';
            } else {
                label.textContent = 'Upstream (host:port)';
                hint.textContent = 'The backend server to proxy requests to';
                input.placeholder = 'localhost:8080';
            }
        });
    },

    openModal(editId = null) {
        const modal = document.getElementById('domain-modal');
        const title = document.getElementById('domain-modal-title');
        const saveBtn = document.getElementById('domain-save-btn');
        const editField = document.getElementById('domain-edit-id');

        if (editId !== null) {
            // Find the domain precisely by its unique ID
            const domain = this.domains.find(d => d.id == editId);
            if (domain) {
                title.textContent = 'Edit Domain';
                saveBtn.textContent = 'Save Changes';
                document.getElementById('domain-name').value = domain.domains ? domain.domains[0] : '';
                document.getElementById('domain-upstream').value = domain.upstream || '';
                document.getElementById('domain-type').value = domain.type || 'reverse_proxy';
                editField.value = editId;
            }
        } else {
            title.textContent = 'Add Domain';
            saveBtn.textContent = 'Add Domain';
            document.getElementById('domain-form').reset();
            editField.value = '';
        }

        modal.style.display = 'flex';
    },

    closeModal() {
        document.getElementById('domain-modal').style.display = 'none';
    },

    async saveDomain() {
        const editId = document.getElementById('domain-edit-id').value;
        const payload = {
            domain: document.getElementById('domain-name').value.trim(),
            upstream: document.getElementById('domain-upstream').value.trim(),
            type: document.getElementById('domain-type').value,
        };

        if (!payload.domain || !payload.upstream) {
            BinaryPanel.toast('Please fill in all fields', 'error');
            return;
        }

        try {
            if (editId) {
                await BinaryPanel.apiRequest(`/api/domains/${editId}`, {
                    method: 'PUT',
                    body: JSON.stringify(payload),
                });
                BinaryPanel.toast('Domain updated successfully', 'success');
            } else {
                await BinaryPanel.apiRequest('/api/domains', {
                    method: 'POST',
                    body: JSON.stringify(payload),
                });
                BinaryPanel.toast('Domain added successfully! SSL will be provisioned automatically.', 'success');
            }

            this.closeModal();
            await this.loadDomains();
        } catch (err) {
            BinaryPanel.toast(err.message, 'error');
        }
    },

    editDomain(id) {
        this.openModal(id);
    },

    async deleteDomain(id, name) {
        const confirmed = await BinaryPanel.confirm(
            'Delete Domain',
            `Are you sure you want to remove "${name}"? The SSL certificate will also be removed.`
        );
        if (!confirmed) return;

        try {
            await BinaryPanel.apiRequest(`/api/domains/${id}`, { method: 'DELETE' });
            BinaryPanel.toast('Domain removed successfully', 'success');
            await this.loadDomains();
        } catch (err) {
            BinaryPanel.toast(err.message, 'error');
        }
    },

    async triggerRestart(id, name) {
        if (!await BinaryPanel.confirm('Restart Process', `Bounce the underlying backend web service for ${name}?`)) return;
        try {
            BinaryPanel.toast(`Restarting daemon securely mapped onto ${name} proxy...`, 'info');
            const resp = await fetch(`/api/domains/${id}/restart`, {
                method: 'POST'
            });
            const data = await resp.json();
            if (resp.ok && data.success) {
                BinaryPanel.toast(`Server bounced natively exactly successfully for ${name}!`, 'success');
            } else {
                throw new Error(data.error || 'Domains daemon restart failed payload');
            }
        } catch (err) {
            BinaryPanel.toast(`Failed mapping restart: ${err.message}`, 'error');
        }
    },

    triggerRestore(id, name) {
        this.pendingRestoreId = id;
        this.pendingRestoreName = name;
        document.getElementById('backup-upload-input').click();
    },

    triggerBackup(id, name) {
        BinaryPanel.toast('Compressing mapping and dynamically extracting ' + name + '...', 'info');
        window.open(`/api/domains/${id}/backup`, '_blank');
    },

    async uploadBackupZip(file, id, name) {
        if (!await BinaryPanel.confirm('Restore Website', `Are you absolutely certain you want to extract and overwrite the remote proxy application "${name}" with this zip backup? Existing deployment files will be replaced securely.`)) return;

        BinaryPanel.toast('Pushing website layout over proxy routing protocol... Please wait', 'info');

        const formData = new FormData();
        formData.append('backup', file);

        try {
            const resp = await fetch(`/api/domains/${id}/restore`, {
                method: 'POST',
                body: formData
            });

            const data = await resp.json();
            if (resp.ok && data.success) {
                BinaryPanel.toast(`Website extracted & integrated cleanly into ${name}! Target proxy restarted completely.`, 'success');
                await this.loadDomains();
            } else {
                throw new Error(data.error || 'Domains daemon extraction failed structurally');
            }
        } catch (err) {
            BinaryPanel.toast(`Failed deploying zip over proxy mapping: ${err.message}`, 'error');
        }
    }
};

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    DomainsModule.init();

    const backupInput = document.getElementById('backup-upload-input');
    if (backupInput) {
        backupInput.addEventListener('change', (e) => {
            const file = e.target.files[0];
            if (!file) return;

            // Enforce zip exclusively structurally
            if (!file.name.toLowerCase().endsWith('.zip')) {
                BinaryPanel.toast('Only .zip backups are currently authorized safely.', 'error');
                backupInput.value = '';
                return;
            }

            if (DomainsModule.pendingRestoreId !== undefined) {
                DomainsModule.uploadBackupZip(file, DomainsModule.pendingRestoreId, DomainsModule.pendingRestoreName);
                DomainsModule.pendingRestoreId = undefined;
            }
            backupInput.value = ''; 
        });
    }
});
