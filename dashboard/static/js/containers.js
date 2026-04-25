/* ═══════════════════════════════════════════════════════════════
   BinaryPanel — Container Management Module
   ═══════════════════════════════════════════════════════════════ */

const ContainersModule = {
    containers: [],

    init() {
        document.getElementById('refresh-containers-btn').addEventListener('click', () => {
            this.loadContainers();
            BinaryPanel.toast('Containers refreshed', 'info');
        });

        document.getElementById('logs-modal-close').addEventListener('click', () => {
            document.getElementById('logs-modal').style.display = 'none';
        });

        // Setup global Zip Unpacker event listener natively
        const uploadInput = document.getElementById('backup-upload-input');
        if (uploadInput) {
            uploadInput.addEventListener('change', async (e) => {
                if (e.target.files.length > 0 && this.pendingRestoreId) {
                    await this.uploadBackupZip(e.target.files[0], this.pendingRestoreId, this.pendingRestoreName);
                    e.target.value = ''; // Reset natively
                }
            });
        }
    },

    pendingRestoreId: null,
    pendingRestoreName: null,

    async loadContainers() {
        try {
            const data = await BinaryPanel.apiRequest('/api/containers');
            if (!data) return;

            this.containers = data.containers || [];
            this.renderContainers();
            this.updateDashboardStat();
        } catch (err) {
            console.error('Failed to load containers:', err);
            document.getElementById('containers-empty').innerHTML = `
                <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="var(--red)" stroke-width="1" opacity="0.5">
                    <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
                </svg>
                <h3>Cannot connect to Docker</h3>
                <p>${BinaryPanel.escapeHtml(err.message)}</p>
            `;
        }
    },

    renderContainers() {
        const grid = document.getElementById('containers-grid');
        const empty = document.getElementById('containers-empty');

        if (!this.containers.length) {
            grid.innerHTML = '';
            grid.appendChild(empty);
            empty.innerHTML = `
                <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" opacity="0.3">
                    <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
                </svg>
                <h3>No containers found</h3>
                <p>Docker is connected but no containers exist yet.</p>
            `;
            return;
        }

        grid.innerHTML = this.containers.map(c => {
            const isRunning = c.state === 'running';
            const ports = (c.ports || [])
                .filter(p => p.public > 0)
                .map(p => `<span class="badge badge-gray">${p.public}→${p.private}/${p.type}</span>`)
                .join('');

            return `
                <div class="container-card">
                    <div class="container-card-header">
                        <div class="container-name">
                            <span class="container-status ${c.state}"></span>
                            ${BinaryPanel.escapeHtml(c.name)}
                        </div>
                        <span class="badge ${isRunning ? 'badge-green' : 'badge-gray'}">${c.state}</span>
                    </div>
                    <div class="container-image" title="${BinaryPanel.escapeHtml(c.image)}">
                        ${BinaryPanel.escapeHtml(c.image)}
                    </div>
                    <div class="container-meta">
                        <span class="badge badge-blue">${BinaryPanel.escapeHtml(c.id)}</span>
                        ${ports}
                    </div>
                    <div class="container-card-actions">
                        ${isRunning ? `
                            <button class="btn btn-outline btn-sm" onclick="ContainersModule.stopContainer('${c.id}', '${BinaryPanel.escapeHtml(c.name)}')">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
                                Stop
                            </button>
                            <button class="btn btn-outline btn-sm" onclick="ContainersModule.restartContainer('${c.id}', '${BinaryPanel.escapeHtml(c.name)}')">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
                                Restart
                            </button>
                        ` : `
                            <button class="btn btn-primary btn-sm" onclick="ContainersModule.startContainer('${c.id}', '${BinaryPanel.escapeHtml(c.name)}')">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                                Start
                            </button>
                        `}
                        <button class="btn btn-ghost btn-sm" onclick="ContainersModule.viewLogs('${c.id}', '${BinaryPanel.escapeHtml(c.name)}')">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                            Logs
                        </button>
                        <button class="btn btn-ghost btn-sm text-red" style="color: var(--red);" onclick="ContainersModule.deleteContainer('${c.id}', '${BinaryPanel.escapeHtml(c.name)}')">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path><line x1="10" y1="11" x2="10" y2="17"></line><line x1="14" y1="11" x2="14" y2="17"></line></svg>
                            Delete
                        </button>
                    </div>
                </div>
            `;
        }).join('');
    },

    updateDashboardStat() {
        const running = this.containers.filter(c => c.state === 'running').length;
        const total = this.containers.length;
        const el = document.getElementById('stat-containers');
        if (el) el.textContent = `${running}/${total}`;
    },

    async startContainer(id, name) {
        try {
            await BinaryPanel.apiRequest(`/api/containers/${id}/start`, { method: 'POST' });
            BinaryPanel.toast(`${name} started successfully`, 'success');
            await this.loadContainers();
        } catch (err) {
            BinaryPanel.toast(`Failed to start ${name}: ${err.message}`, 'error');
        }
    },

    async stopContainer(id, name) {
        const confirmed = await BinaryPanel.confirm('Stop Container', `Are you sure you want to stop "${name}"?`);
        if (!confirmed) return;

        try {
            await BinaryPanel.apiRequest(`/api/containers/${id}/stop`, { method: 'POST' });
            BinaryPanel.toast(`${name} stopped`, 'success');
            await this.loadContainers();
        } catch (err) {
            BinaryPanel.toast(`Failed to stop ${name}: ${err.message}`, 'error');
        }
    },

    async restartContainer(id, name) {
        try {
            await BinaryPanel.apiRequest(`/api/containers/${id}/restart`, { method: 'POST' });
            BinaryPanel.toast(`${name} restarted`, 'success');
            await this.loadContainers();
        } catch (err) {
            BinaryPanel.toast(`Failed to restart ${name}: ${err.message}`, 'error');
        }
    },

    async deleteContainer(id, name) {
        const confirmed = await BinaryPanel.confirm('Delete Container', `Are you absolutely sure you want to permanently delete "${name}"? This action cannot be undone and non-persistent data will be lost.`);
        if (!confirmed) return;

        try {
            await BinaryPanel.apiRequest(`/api/containers/${id}`, { method: 'DELETE' });
            BinaryPanel.toast(`${name} deleted permanently`, 'success');
            await this.loadContainers();
        } catch (err) {
            BinaryPanel.toast(`Failed to delete ${name}: ${err.message}`, 'error');
        }
    },

    async viewLogs(id, name) {
        document.getElementById('logs-modal-title').textContent = `Logs — ${name}`;
        document.getElementById('log-content').textContent = 'Loading logs...';
        document.getElementById('logs-modal').style.display = 'flex';

        try {
            const data = await BinaryPanel.apiRequest(`/api/containers/${id}/logs?lines=200`);
            document.getElementById('log-content').textContent = data.logs || 'No logs available.';
            // Auto-scroll to bottom
            const viewer = document.getElementById('log-viewer');
            viewer.scrollTop = viewer.scrollHeight;
        } catch (err) {
            document.getElementById('log-content').textContent = `Error: ${err.message}`;
        }
    }
};

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => ContainersModule.init());
