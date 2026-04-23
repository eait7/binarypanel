/* ═══════════════════════════════════════════════════════════════
   BinaryPanel — System Monitoring Module
   ═══════════════════════════════════════════════════════════════ */

const SystemModule = {
    GAUGE_CIRCUMFERENCE: 326.73, // 2 * π * 52

    async loadStats() {
        try {
            const stats = await BinaryPanel.apiRequest('/api/system/stats');
            if (!stats) return;

            this.updateGauges(stats);
            this.updateDashboardStats(stats);
            this.updateSystemInfo(stats);
        } catch (err) {
            console.error('Failed to load system stats:', err);
        }
    },

    updateGauges(stats) {
        // CPU gauge
        this.setGauge('gauge-cpu-fill', 'gauge-cpu-value', stats.cpu_usage, '%');

        // Memory gauge
        this.setGauge('gauge-mem-fill', 'gauge-mem-value', stats.mem_percent, '%');

        // Disk gauge
        this.setGauge('gauge-disk-fill', 'gauge-disk-value', stats.disk_percent, '%');
    },

    setGauge(fillId, valueId, percent, suffix) {
        const fill = document.getElementById(fillId);
        const value = document.getElementById(valueId);
        if (!fill || !value) return;

        const clampedPercent = Math.min(100, Math.max(0, percent));
        const offset = this.GAUGE_CIRCUMFERENCE - (this.GAUGE_CIRCUMFERENCE * clampedPercent / 100);
        fill.style.strokeDashoffset = offset;

        value.textContent = `${Math.round(clampedPercent)}${suffix}`;

        // Color change for high usage
        if (clampedPercent > 90) {
            fill.style.stroke = 'var(--red)';
        } else if (clampedPercent > 75) {
            fill.style.stroke = 'var(--amber)';
        }
    },

    updateDashboardStats(stats) {
        const uptimeEl = document.getElementById('stat-uptime');
        const cpuEl = document.getElementById('stat-cpu');

        if (uptimeEl) uptimeEl.textContent = stats.uptime || '—';
        if (cpuEl) cpuEl.textContent = `${Math.round(stats.cpu_usage)}%`;
    },

    updateSystemInfo(stats) {
        // Server Info
        this.setText('sys-hostname', stats.hostname);
        this.setText('sys-os', `${stats.os}/${stats.arch}`);
        this.setText('sys-arch', stats.arch);
        this.setText('sys-cpus', stats.num_cpu);
        this.setText('sys-uptime', stats.uptime);

        // Load Average
        this.setText('load-1m', stats.load_avg_1 ? stats.load_avg_1.toFixed(2) : '—');
        this.setText('load-5m', stats.load_avg_5 ? stats.load_avg_5.toFixed(2) : '—');
        this.setText('load-15m', stats.load_avg_15 ? stats.load_avg_15.toFixed(2) : '—');

        // Memory bar
        const memBar = document.getElementById('mem-bar-fill');
        if (memBar) memBar.style.width = `${stats.mem_percent}%`;
        this.setText('mem-used-label', `${BinaryPanel.formatBytes(stats.mem_used)} used`);
        this.setText('mem-total-label', `${BinaryPanel.formatBytes(stats.mem_total)} total`);

        // Disk bar
        const diskBar = document.getElementById('disk-bar-fill');
        if (diskBar) diskBar.style.width = `${stats.disk_percent}%`;
        this.setText('disk-used-label', `${BinaryPanel.formatBytes(stats.disk_used)} used`);
        this.setText('disk-total-label', `${BinaryPanel.formatBytes(stats.disk_total)} total`);
    },

    setText(id, value) {
        const el = document.getElementById(id);
        if (el) el.textContent = value ?? '—';
    },
};
