class SettingsManager {
    constructor() {
        this.form = document.getElementById('email-settings-form');
        this.providerSelect = document.getElementById('smtp-provider');
        this.hostInput = document.getElementById('smtp-host');
        this.portInput = document.getElementById('smtp-port');
        this.secureCheck = document.getElementById('smtp-secure');
        this.fromInput = document.getElementById('smtp-from');
        this.usernameInput = document.getElementById('smtp-username');
        this.passwordInput = document.getElementById('smtp-password');
        this.testBtn = document.getElementById('test-email-btn');

        this.init();
    }

    async init() {
        if (!this.form) return;

        // Populate providers
        for (const [key, p] of Object.entries(SMTPProviders)) {
            const opt = document.createElement('option');
            opt.value = key;
            opt.textContent = p.name;
            this.providerSelect.appendChild(opt);
        }

        this.providerSelect.addEventListener('change', () => this.handleProviderChange());
        this.form.addEventListener('submit', (e) => this.handleSave(e));
        this.testBtn.addEventListener('click', () => this.handleTest());

        this.loadSettings();
    }

    handleProviderChange() {
        const val = this.providerSelect.value;
        if (val === 'custom') {
            this.hostInput.readOnly = false;
            this.portInput.readOnly = false;
            return;
        }

        const p = SMTPProviders[val];
        if (p) {
            this.hostInput.value = p.host;
            this.portInput.value = p.port;
            this.secureCheck.checked = p.secure;
            // Optionally set readonly for preset fields, but UX usually allows tweaking
        }
    }

    async loadSettings() {
        try {
            const cfg = await BinaryPanel.api('/api/settings/email');
            if (cfg) {
                if (cfg.provider) this.providerSelect.value = cfg.provider;
                this.hostInput.value = cfg.host || '';
                this.portInput.value = cfg.port || '';
                this.secureCheck.checked = cfg.secure || false;
                this.usernameInput.value = cfg.username || '';
                this.passwordInput.value = cfg.password || ''; // Will be ********
                this.fromInput.value = cfg.from || '';
            }
        } catch (e) {
            BinaryPanel.toast('Failed to load email configuration', 'error');
        }
    }

    async handleSave(e) {
        e.preventDefault();
        const submitBtn = this.form.querySelector('button[type="submit"]');
        const prevText = submitBtn.textContent;
        submitBtn.textContent = 'Saving...';
        submitBtn.disabled = true;

        const payload = {
            provider: this.providerSelect.value,
            host: this.hostInput.value,
            port: this.portInput.value,
            secure: this.secureCheck.checked,
            username: this.usernameInput.value,
            password: this.passwordInput.value, // Backend ignores if '********'
            from: this.fromInput.value
        };

        try {
            await BinaryPanel.api('/api/settings/email', {
                method: 'PUT',
                body: JSON.stringify(payload)
            });
            BinaryPanel.toast('Settings saved successfully');
        } catch (err) {
            BinaryPanel.toast(err.message, 'error');
        } finally {
            submitBtn.textContent = prevText;
            submitBtn.disabled = false;
        }
    }

    async handleTest() {
        const email = prompt("Enter an email address to send the test message to:");
        if (!email) return;

        const originalText = this.testBtn.textContent;
        this.testBtn.textContent = 'Sending...';
        this.testBtn.disabled = true;

        try {
            await BinaryPanel.api('/api/settings/email/test', {
                method: 'POST',
                body: JSON.stringify({ to: email })
            });
            BinaryPanel.toast('Test email sent successfully! Please check your inbox.');
        } catch (err) {
            BinaryPanel.toast('Failed to send email: ' + err.message, 'error');
        } finally {
            this.testBtn.textContent = originalText;
            this.testBtn.disabled = false;
        }
    }
}

document.addEventListener('DOMContentLoaded', () => {
    new SettingsManager();
});
