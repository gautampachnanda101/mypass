// API client
let TOKEN = sessionStorage.getItem('vaultx_token') || null;

const api = {
    get headers() {
        return {
            'X-Vaultx-Token': TOKEN,
            'Content-Type': 'application/json'
        };
    },

    async authenticateWithTouchID() {
        const response = await fetch('/auth/touchid', {
            method: 'POST'
        });
        if (!response.ok) {
            throw new Error('Touch ID authentication failed or was cancelled');
        }
        const data = await response.json();
        TOKEN = data.token;
        sessionStorage.setItem('vaultx_token', TOKEN);
        return data;
    },

    async listSecrets(prefix = '') {
        const url = prefix ? `/v1/list?prefix=${encodeURIComponent(prefix)}` : '/v1/list';
        const response = await fetch(url, {
            headers: this.headers
        });
        if (!response.ok) {
            if (response.status === 401) {
                // Token expired or server restarted - clear and re-authenticate
                TOKEN = null;
                sessionStorage.removeItem('vaultx_token');
                throw new Error('Session expired - please refresh the page');
            }
            throw new Error(`Failed to list secrets: ${response.statusText}`);
        }
        return response.json();
    },

    async getSecret(path) {
        const response = await fetch(`/v1/secret?path=${encodeURIComponent(path)}`, {
            headers: this.headers
        });
        if (!response.ok) {
            if (response.status === 401) {
                TOKEN = null;
                sessionStorage.removeItem('vaultx_token');
                throw new Error('Session expired - please refresh the page');
            }
            throw new Error(`Failed to get secret: ${response.statusText}`);
        }
        return response.json();
    },

    async resolveEnv(envContent) {
        const response = await fetch('/v1/resolve', {
            method: 'POST',
            headers: this.headers,
            body: envContent
        });
        if (!response.ok) {
            if (response.status === 401) {
                TOKEN = null;
                sessionStorage.removeItem('vaultx_token');
                throw new Error('Session expired - please refresh the page');
            }
            const error = await response.json();
            throw new Error(error.error || 'Failed to resolve env file');
        }
        return response.json();
    },

    async getAuditLog(limit = 100) {
        const response = await fetch(`/v1/audit?limit=${limit}`, {
            headers: this.headers
        });
        if (!response.ok) {
            if (response.status === 401) {
                TOKEN = null;
                sessionStorage.removeItem('vaultx_token');
                throw new Error('Session expired - please refresh the page');
            }
            throw new Error(`Failed to get audit log: ${response.statusText}`);
        }
        return response.json();
    }
};

// UI State
const state = {
    secrets: [],
    filteredSecrets: [],
    currentTab: 'secrets',
    searchQuery: ''
};

// DOM Elements
const elements = {
    tabs: document.querySelectorAll('.tab'),
    tabContents: document.querySelectorAll('.tab-content'),
    searchInput: document.getElementById('search-input'),
    refreshBtn: document.getElementById('refresh-btn'),
    addSecretBtn: document.getElementById('add-secret-btn'),
    secretsList: document.getElementById('secrets-list'),
    modal: document.getElementById('secret-modal'),
    modalClose: document.querySelector('.modal-close'),
    cancelBtn: document.getElementById('cancel-btn'),
    saveBtn: document.getElementById('save-btn'),
    secretPath: document.getElementById('secret-path'),
    secretValue: document.getElementById('secret-value'),
    toggleVisibility: document.getElementById('toggle-visibility'),
    envInput: document.getElementById('env-input'),
    resolveBtn: document.getElementById('resolve-btn'),
    resolveOutput: document.getElementById('resolve-output'),
    resolvedContent: document.getElementById('resolved-content'),
    copyResolvedBtn: document.getElementById('copy-resolved-btn')
};

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    // Check if authenticated
    if (!TOKEN) {
        await showTouchIDPrompt();
    }
    initTabs();
    initSecrets();
    initResolve();
    initAudit();
    initModal();
});

// Touch ID Authentication
async function showTouchIDPrompt() {
    return new Promise((resolve, reject) => {
        const authModal = document.createElement('div');
        authModal.className = 'modal active';
        authModal.id = 'touchid-modal';
        authModal.innerHTML = `
            <div class="modal-content">
                <div class="modal-header">
                    <h3>🔐 Authentication Required</h3>
                </div>
                <div class="modal-body" style="text-align: center; padding: 2rem;">
                    <div style="font-size: 4rem; margin-bottom: 1rem;">👆</div>
                    <h3 style="margin-bottom: 1rem;">Authenticate with Touch ID</h3>
                    <p style="color: #6b7280; margin-bottom: 2rem;">
                        Place your finger on the Touch ID sensor to access vaultx
                    </p>
                    <button id="touchid-auth-btn" class="btn btn-primary" style="font-size: 1rem; padding: 0.75rem 2rem;">
                        Authenticate
                    </button>
                    <div id="auth-error" class="alert alert-error" style="display: none; margin-top: 1rem;"></div>
                </div>
            </div>
        `;
        document.body.appendChild(authModal);

        const authBtn = document.getElementById('touchid-auth-btn');
        const authError = document.getElementById('auth-error');

        authBtn.addEventListener('click', async () => {
            try {
                authBtn.disabled = true;
                authBtn.textContent = 'Authenticating...';
                authError.style.display = 'none';

                await api.authenticateWithTouchID();
                
                // Authentication successful
                authModal.remove();
                showAlert('Authenticated with Touch ID', 'success');
                resolve();
            } catch (error) {
                authError.textContent = error.message;
                authError.style.display = 'block';
                authBtn.disabled = false;
                authBtn.textContent = 'Retry Authentication';
            }
        });

        // Auto-trigger on modal appearance
        setTimeout(() => authBtn.click(), 500);
    });
}

// Tab Management
function initTabs() {
    elements.tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            const tabName = tab.dataset.tab;
            switchTab(tabName);
        });
    });
}

function switchTab(tabName) {
    state.currentTab = tabName;
    
    elements.tabs.forEach(tab => {
        tab.classList.toggle('active', tab.dataset.tab === tabName);
    });
    
    elements.tabContents.forEach(content => {
        content.classList.toggle('active', content.id === `${tabName}-tab`);
    });
}

// Secrets Management
function initSecrets() {
    elements.searchInput.addEventListener('input', (e) => {
        state.searchQuery = e.target.value.toLowerCase();
        filterAndRenderSecrets();
    });

    elements.refreshBtn.addEventListener('click', loadSecrets);
    elements.addSecretBtn.addEventListener('click', () => {
        showAlert('Secret creation requires vaultx CLI', 'warning');
        showInfo();
    });

    loadSecrets();
}

async function loadSecrets() {
    try {
        elements.secretsList.innerHTML = '<div class="loading">Loading secrets...</div>';
        const secrets = await api.listSecrets();
        state.secrets = Array.isArray(secrets) ? secrets : [];
        filterAndRenderSecrets();
    } catch (error) {
        const isSessionExpired = error.message.includes('Session expired');
        const actionButton = isSessionExpired 
            ? '<button class="btn btn-primary" onclick="location.reload()">🔄 Refresh Page</button>'
            : '<button class="btn btn-primary" onclick="loadSecrets()">Retry</button>';
        
        elements.secretsList.innerHTML = `
            <div class="empty-state">
                <h3>⚠️ Failed to Load Secrets</h3>
                <p>${escapeHtml(error.message)}</p>
                <p style="margin-top: 1rem;">
                    ${actionButton}
                </p>
            </div>
        `;
    }
}

function filterAndRenderSecrets() {
    if (state.searchQuery) {
        state.filteredSecrets = state.secrets.filter(s => 
            s.Key.toLowerCase().includes(state.searchQuery)
        );
    } else {
        state.filteredSecrets = state.secrets;
    }
    renderSecrets();
}

function renderSecrets() {
    if (state.filteredSecrets.length === 0) {
        const message = state.searchQuery 
            ? `No secrets matching "${escapeHtml(state.searchQuery)}"`
            : 'No secrets found. Use <code>vaultx set path value</code> to add secrets.';
        
        elements.secretsList.innerHTML = `
            <div class="empty-state">
                <h3>📭 No Secrets</h3>
                <p>${message}</p>
            </div>
        `;
        return;
    }

    const html = state.filteredSecrets.map(secret => `
        <div class="secret-item">
            <div class="secret-info">
                <div class="secret-path">${escapeHtml(secret.Key)}</div>
                <div class="secret-meta">
                    <span>Provider: ${escapeHtml(secret.Provider || 'local')}</span>
                    ${secret.UpdatedAt ? `<span>Updated: ${formatDate(secret.UpdatedAt)}</span>` : ''}
                </div>
            </div>
            <div class="secret-actions">
                <button class="btn btn-secondary" onclick="viewSecret('${escapeHtml(secret.Key)}')">
                    👁️ View
                </button>
            </div>
        </div>
    `).join('');

    elements.secretsList.innerHTML = html;
}

async function viewSecret(path) {
    try {
        const result = await api.getSecret(path);
        showSecretModal(path, result.value);
    } catch (error) {
        showAlert(`Failed to retrieve secret: ${error.message}`, 'error');
    }
}

function showSecretModal(path, value) {
    const modal = document.createElement('div');
    modal.className = 'modal active';
    modal.innerHTML = `
        <div class="modal-content">
            <div class="modal-header">
                <h3>Secret Value</h3>
                <button class="modal-close" onclick="this.closest('.modal').remove()">&times;</button>
            </div>
            <div class="modal-body">
                <div class="form-group">
                    <label>Path</label>
                    <input type="text" class="form-input" value="${escapeHtml(path)}" readonly>
                </div>
                <div class="form-group">
                    <label>Value</label>
                    <textarea class="form-input" rows="3" readonly style="font-family: monospace;">${escapeHtml(value)}</textarea>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="copyToClipboard('${escapeHtml(value)}')">
                    📋 Copy Value
                </button>
                <button class="btn btn-primary" onclick="this.closest('.modal').remove()">
                    Close
                </button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
}

// Resolve Tab
function initResolve() {
    elements.resolveBtn.addEventListener('click', resolveEnvFile);
    elements.copyResolvedBtn.addEventListener('click', copyResolvedContent);
}

async function resolveEnvFile() {
    const envContent = elements.envInput.value.trim();
    
    if (!envContent) {
        showAlert('Please enter vaultx.env content', 'warning');
        return;
    }

    try {
        elements.resolveBtn.disabled = true;
        elements.resolveBtn.textContent = 'Resolving...';
        
        const resolved = await api.resolveEnv(envContent);
        
        const output = Object.entries(resolved)
            .map(([key, value]) => `${key}=${value}`)
            .join('\n');
        
        elements.resolvedContent.textContent = output;
        elements.resolveOutput.style.display = 'block';
        
        showAlert('Successfully resolved all secrets', 'success');
    } catch (error) {
        showAlert(`Resolution failed: ${error.message}`, 'error');
        elements.resolveOutput.style.display = 'none';
    } finally {
        elements.resolveBtn.disabled = false;
        elements.resolveBtn.textContent = 'Resolve Secrets';
    }
}

function copyResolvedContent() {
    copyToClipboard(elements.resolvedContent.textContent);
}

// Audit Log Management
function initAudit() {
    // Tab switching to audit will trigger load
    elements.tabs.forEach(tab => {
        if (tab.dataset.tab === 'audit') {
            tab.addEventListener('click', loadAuditLog);
        }
    });
}

async function loadAuditLog() {
    const auditLogElement = document.getElementById('audit-log');
    if (!auditLogElement) return;

    try {
        auditLogElement.innerHTML = '<div class="loading">Loading audit events...</div>';
        const events = await api.getAuditLog(100);
        
        if (!events || events.length === 0) {
            auditLogElement.innerHTML = `
                <div class="empty-state">
                    <h3>📝 No Audit Events Yet</h3>
                    <p>Security events will appear here as you use vaultx</p>
                </div>
            `;
            return;
        }

        renderAuditLog(events, auditLogElement);
    } catch (error) {
        auditLogElement.innerHTML = `
            <div class="empty-state">
                <h3>⚠️ Failed to Load Audit Log</h3>
                <p>${escapeHtml(error.message)}</p>
                <p style="margin-top: 1rem;">
                    <button class="btn btn-primary" onclick="loadAuditLog()">Retry</button>
                </p>
            </div>
        `;
    }
}

function renderAuditLog(events, container) {
    const eventRows = events.map(event => {
        const statusClass = event.success ? 'success' : 'error';
        const statusIcon = event.success ? '✅' : '❌';
        const timestamp = formatDate(event.timestamp);
        const path = event.path ? escapeHtml(event.path) : '—';
        const error = event.error ? `<br><small style="color: var(--danger-color);">${escapeHtml(event.error)}</small>` : '';
        
        return `
            <tr class="audit-row audit-${statusClass}">
                <td><span class="audit-status">${statusIcon}</span></td>
                <td>${timestamp}</td>
                <td><code>${escapeHtml(event.action)}</code></td>
                <td><code>${path}</code></td>
                <td><code style="font-size: 0.75rem;">${escapeHtml(event.remote_addr)}</code></td>
                <td>${error}</td>
            </tr>
        `;
    }).join('');

    container.innerHTML = `
        <div style="overflow-x: auto;">
            <table class="audit-table">
                <thead>
                    <tr>
                        <th style="width: 40px;"></th>
                        <th style="width: 180px;">Timestamp</th>
                        <th style="width: 150px;">Action</th>
                        <th>Path</th>
                        <th style="width: 150px;">Remote Addr</th>
                        <th style="width: 80px;">Details</th>
                    </tr>
                </thead>
                <tbody>
                    ${eventRows}
                </tbody>
            </table>
        </div>
        <div style="margin-top: 1rem; text-align: center;">
            <button class="btn btn-secondary" onclick="loadAuditLog()">🔄 Refresh</button>
        </div>
    `;
}

// Modal Management
function initModal() {
    elements.modalClose.addEventListener('click', closeModal);
    elements.cancelBtn.addEventListener('click', closeModal);
    
    elements.toggleVisibility.addEventListener('click', () => {
        const input = elements.secretValue;
        if (input.type === 'password') {
            input.type = 'text';
            elements.toggleVisibility.textContent = 'Hide';
        } else {
            input.type = 'password';
            elements.toggleVisibility.textContent = 'Show';
        }
    });

    // Close modal on outside click
    elements.modal.addEventListener('click', (e) => {
        if (e.target === elements.modal) {
            closeModal();
        }
    });
}

function openModal() {
    elements.modal.classList.add('active');
    elements.secretPath.value = '';
    elements.secretValue.value = '';
    elements.secretValue.type = 'password';
    elements.toggleVisibility.textContent = 'Show';
}

function closeModal() {
    elements.modal.classList.remove('active');
}

function showInfo() {
    const infoModal = document.createElement('div');
    infoModal.className = 'modal active';
    infoModal.innerHTML = `
        <div class="modal-content">
            <div class="modal-header">
                <h3>ℹ️ vaultx Web UI Help</h3>
                <button class="modal-close" onclick="this.closest('.modal').remove()">&times;</button>
            </div>
            <div class="modal-body" style="max-height: 500px; overflow-y: auto;">
                <h4 style="margin-top: 0;">Managing Secrets</h4>
                <p>For security, secrets can only be created or modified using the vaultx CLI:</p>
                <pre style="background: #f3f4f6; padding: 1rem; border-radius: 0.375rem; font-size: 0.875rem;">vaultx set myapp/db_password "secret-value"
vaultx set myapp/api_key "sk-live-abc123"
vaultx delete myapp/old_key
vaultx list myapp/</pre>
                
                <h4 style="margin-top: 1.5rem;">Viewing Secrets</h4>
                <p style="margin-bottom: 0.5rem;">Click the 👁️ View button to see the actual secret value. Use the search box to filter by path.</p>
                
                <h4 style="margin-top: 1.5rem;">Resolving vaultx.env Files</h4>
                <p style="margin-bottom: 0.5rem;">Switch to the <strong>Resolve</strong> tab to paste your vaultx.env file:</p>
                <pre style="background: #f3f4f6; padding: 1rem; border-radius: 0.375rem; font-size: 0.875rem;">DB_PASSWORD=vault:local/myapp/db_password
API_KEY=vault:local/myapp/api_key
PORT=3000</pre>
                <p style="margin-top: 0.5rem;">Click <strong>Resolve Secrets</strong> to see all vault: references replaced with actual values.</p>
                
                <h4 style="margin-top: 1.5rem;">Security Features</h4>
                <ul style="margin-left: 1.5rem; margin-bottom: 0;">
                    <li>Touch ID authentication required</li>
                    <li>Rate limiting (10 req/s, burst 50)</li>
                    <li>Session token in browser memory only</li>
                    <li>Path traversal protection</li>
                    <li>Audit logging of all events</li>
                </ul>
                
                <h4 style="margin-top: 1.5rem;">CLI Usage</h4>
                <p style="margin-bottom: 0.5rem;">Run the daemon:</p>
                <pre style="background: #f3f4f6; padding: 1rem; border-radius: 0.375rem; font-size: 0.875rem;">vaultx serve              # default port 7474
vaultx serve --port 8080  # custom port</pre>
                
                <p style="margin-top: 1rem; font-size: 0.875rem; color: #6b7280;">
                    📖 Full documentation: <a href="https://github.com/gautampachnanda101/vaultx" target="_blank" rel="noopener">github.com/gautampachnanda101/vaultx</a>
                </p>
            </div>
            <div class="modal-footer">
                <button class="btn btn-primary" onclick="this.closest('.modal').remove()">
                    Got it
                </button>
            </div>
        </div>
    `;
    document.body.appendChild(infoModal);
}

// Utilities
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatDate(dateString) {
    try {
        const date = new Date(dateString);
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
    } catch {
        return dateString;
    }
}

function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        showAlert('Copied to clipboard', 'success');
    }).catch(() => {
        showAlert('Failed to copy to clipboard', 'error');
    });
}

function showAlert(message, type = 'info') {
    const alert = document.createElement('div');
    alert.className = `alert alert-${type}`;
    alert.textContent = message;
    alert.style.cssText = 'position: fixed; top: 1rem; right: 1rem; z-index: 9999; max-width: 400px;';
    
    document.body.appendChild(alert);
    
    setTimeout(() => {
        alert.style.opacity = '0';
        alert.style.transition = 'opacity 0.3s';
        setTimeout(() => alert.remove(), 300);
    }, 3000);
}
