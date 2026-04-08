// Mandau Dashboard JavaScript

class MandauDashboard {
    constructor() {
        this.connected = false;
        this.serverAddress = '';
        this.apiBase = '/api/v1';
        this.refreshInterval = null;
        this.token = localStorage.getItem('mandau-token');
        this.username = localStorage.getItem('mandau-username');
        this.init();
    }

    init() {
        // Check if user is logged in
        if (!this.token) {
            window.location.href = '/login';
            return;
        }

        this.bindEvents();
        this.loadSavedConfig();
        this.updateConnectionStatus();
        this.updateUserInfo();
    }

    bindEvents() {
        // Navigation
        document.querySelectorAll('[data-page]').forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const page = e.currentTarget.dataset.page;
                this.navigate(page);
            });
        });

        // Connection
        document.getElementById('btn-connect').addEventListener('click', () => {
            this.showConnectionModal();
        });

        document.getElementById('btn-modal-connect').addEventListener('click', () => {
            this.connect();
        });

        // Refresh buttons
        document.getElementById('btn-refresh').addEventListener('click', () => {
            this.refreshDashboard();
        });

        document.getElementById('btn-refresh-agents').addEventListener('click', () => {
            this.loadAgents();
        });

        document.getElementById('btn-refresh-stacks').addEventListener('click', () => {
            this.loadStacks();
        });

        document.getElementById('btn-refresh-containers').addEventListener('click', () => {
            this.loadContainers();
        });

        document.getElementById('btn-refresh-operations').addEventListener('click', () => {
            this.loadOperations();
        });

        // Auto-refresh every 30 seconds
        this.refreshInterval = setInterval(() => {
            if (this.connected) {
                this.refreshDashboard();
            }
        }, 30000);
    }

    navigate(page) {
        // Hide all pages
        document.querySelectorAll('.page-content').forEach(el => {
            el.style.display = 'none';
        });

        // Show target page
        const pageEl = document.getElementById(`page-${page}`);
        if (pageEl) {
            pageEl.style.display = 'block';
        }

        // Update nav links
        document.querySelectorAll('[data-page]').forEach(link => {
            link.classList.remove('active');
        });

        document.querySelector(`[data-page="${page}"]`).classList.add('active');

        // Load page data
        if (this.connected) {
            switch (page) {
                case 'dashboard':
                    this.refreshDashboard();
                    break;
                case 'agents':
                    this.loadAgents();
                    break;
                case 'stacks':
                    this.loadStacks();
                    break;
                case 'containers':
                    this.loadContainers();
                    break;
                case 'operations':
                    this.loadOperations();
                    break;
                case 'logs':
                    this.loadLogs();
                    break;
            }
        }
    }

    showConnectionModal() {
        const modal = new bootstrap.Modal(document.getElementById('connectionModal'));
        modal.show();
    }

    async connect() {
        const address = document.getElementById('server-address').value;
        if (!address) {
            this.showToast('Please enter server address', 'error');
            return;
        }

        this.serverAddress = address;
        this.saveConfig();

        // Try to connect
        try {
            // For now, simulate connection - in production, this would call the actual API
            this.connected = true;
            this.updateConnectionStatus();
            
            // Close modal
            const modal = bootstrap.Modal.getInstance(document.getElementById('connectionModal'));
            modal.hide();

            this.showToast('Connected to Mandau Core', 'success');
            this.refreshDashboard();
        } catch (error) {
            this.showToast(`Connection failed: ${error.message}`, 'error');
        }
    }

    disconnect() {
        this.connected = false;
        this.updateConnectionStatus();
        this.showToast('Disconnected from Mandau Core', 'warning');
    }

    updateConnectionStatus() {
        const statusEl = document.getElementById('connection-status');
        if (this.connected) {
            statusEl.innerHTML = '<i class="fas fa-circle text-success"></i> Connected';
            document.getElementById('btn-connect').innerHTML = '<i class="fas fa-unlink"></i> Disconnect';
            document.getElementById('btn-connect').onclick = () => this.disconnect();
        } else {
            statusEl.innerHTML = '<i class="fas fa-circle text-danger"></i> Disconnected';
            document.getElementById('btn-connect').innerHTML = '<i class="fas fa-plug"></i> Connect';
            document.getElementById('btn-connect').onclick = () => this.showConnectionModal();
        }
    }

    updateUserInfo() {
        const nav = document.querySelector('.navbar-nav');
        if (nav && this.username) {
            const userInfo = document.createElement('li');
            userInfo.className = 'nav-item';
            userInfo.innerHTML = `
                <span class="nav-link text-light">
                    <i class="fas fa-user-circle"></i> ${this.username}
                </span>
            `;
            nav.appendChild(userInfo);
        }
    }

    getAuthHeaders() {
        return {
            'Authorization': `Bearer ${this.token}`,
            'Content-Type': 'application/json'
        };
    }

    logout() {
        localStorage.removeItem('mandau-token');
        localStorage.removeItem('mandau-username');
        localStorage.removeItem('mandau-roles');
        window.location.href = '/login';
    }

    saveConfig() {
        localStorage.setItem('mandau-server', this.serverAddress);
    }

    loadSavedConfig() {
        const saved = localStorage.getItem('mandau-server');
        if (saved) {
            this.serverAddress = saved;
            document.getElementById('server-address').value = saved;
        }
    }

    async refreshDashboard() {
        if (!this.connected) return;

        try {
            await Promise.all([
                this.loadAgents(),
                this.loadStacks(),
                this.loadContainers(),
                this.loadOperations()
            ]);
        } catch (error) {
            console.error('Failed to refresh dashboard:', error);
        }
    }

    async loadAgents() {
        try {
            const response = await fetch('/api/v1/agents', {
                headers: this.getAuthHeaders()
            });
            
            // Check for 401 Unauthorized
            if (response.status === 401) {
                localStorage.removeItem('mandau-token');
                window.location.href = '/login';
                return;
            }

            const result = await response.json();
            
            if (!result.success) {
                throw new Error(result.error || 'Failed to load agents');
            }

            const agents = result.data || [];
            this.renderAgents(agents);
            this.updateAgentHealth(agents);
            document.getElementById('stat-agents').textContent = agents.length;
            document.getElementById('stat-agents-online').textContent = 
                `${agents.filter(a => a.status === 'online').length} online`;
        } catch (error) {
            console.error('Failed to load agents:', error);
            this.showToast(`Failed to load agents: ${error.message}`, 'error');
        }
    }

    renderAgents(agents) {
        const container = document.getElementById('agents-list');
        
        if (agents.length === 0) {
            container.innerHTML = '<div class="col-12"><div class="alert alert-info">No agents found</div></div>';
            return;
        }

        container.innerHTML = agents.map(agent => `
            <div class="col-md-6 col-lg-4 mb-3">
                <div class="card agent-card ${agent.status}">
                    <div class="card-header d-flex justify-content-between align-items-center">
                        <h6 class="mb-0">${agent.hostname}</h6>
                        <span class="status-badge ${agent.status}">${agent.status}</span>
                    </div>
                    <div class="card-body">
                        <p class="mb-1"><strong>ID:</strong> ${agent.id}</p>
                        <p class="mb-1"><strong>Last Seen:</strong> ${this.formatTime(agent.lastSeen)}</p>
                        <p class="mb-2"><strong>Stacks:</strong> ${agent.stacks.join(', ') || 'None'}</p>
                        <div class="mt-2">
                            ${agent.capabilities.map(cap => `<span class="badge bg-secondary me-1">${cap}</span>`).join('')}
                        </div>
                    </div>
                    <div class="card-footer">
                        <button class="btn btn-sm btn-outline-primary me-1" onclick="dashboard.viewAgent('${agent.id}')">
                            <i class="fas fa-eye"></i> View
                        </button>
                        <button class="btn btn-sm btn-outline-danger" onclick="dashboard.removeAgent('${agent.id}')">
                            <i class="fas fa-trash"></i> Remove
                        </button>
                    </div>
                </div>
            </div>
        `).join('');
    }

    updateAgentHealth(agents) {
        const tbody = document.getElementById('agent-health-table');
        tbody.innerHTML = agents.map(agent => `
            <tr>
                <td>${agent.id}</td>
                <td>${agent.hostname}</td>
                <td><span class="badge bg-${agent.status === 'online' ? 'success' : 'danger'}">${agent.status}</span></td>
                <td>${this.formatTime(agent.lastSeen)}</td>
                <td>${agent.capabilities.map(c => `<span class="badge bg-info me-1">${c}</span>`).join('')}</td>
            </tr>
        `).join('');
    }

    async loadStacks() {
        const stacks = [
            {
                id: 'stack-1',
                name: 'web-app',
                agentId: 'agent-001',
                state: 'running',
                services: ['nginx', 'app', 'redis'],
                createdAt: new Date(Date.now() - 86400000).toISOString()
            },
            {
                id: 'stack-2',
                name: 'api-service',
                agentId: 'agent-001',
                state: 'running',
                services: ['api', 'postgres'],
                createdAt: new Date(Date.now() - 172800000).toISOString()
            }
        ];

        this.renderStacks(stacks);
        document.getElementById('stat-stacks').textContent = stacks.length;
    }

    renderStacks(stacks) {
        const container = document.getElementById('stacks-list');
        
        if (stacks.length === 0) {
            container.innerHTML = '<div class="col-12"><div class="alert alert-info">No stacks found</div></div>';
            return;
        }

        container.innerHTML = stacks.map(stack => `
            <div class="col-md-6 col-lg-4 mb-3">
                <div class="card stack-card ${stack.state}">
                    <div class="card-header d-flex justify-content-between align-items-center">
                        <h6 class="mb-0">${stack.name}</h6>
                        <span class="badge bg-${stack.state === 'running' ? 'success' : 'danger'}">${stack.state}</span>
                    </div>
                    <div class="card-body">
                        <p class="mb-1"><strong>Agent:</strong> ${stack.agentId}</p>
                        <p class="mb-1"><strong>Services:</strong> ${stack.services.join(', ')}</p>
                        <p class="mb-2"><strong>Created:</strong> ${this.formatTime(stack.createdAt)}</p>
                    </div>
                    <div class="card-footer">
                        <button class="btn btn-sm btn-outline-primary me-1" onclick="dashboard.viewStack('${stack.id}')">
                            <i class="fas fa-eye"></i> View
                        </button>
                        <button class="btn btn-sm btn-outline-danger" onclick="dashboard.removeStack('${stack.id}')">
                            <i class="fas fa-trash"></i> Remove
                        </button>
                    </div>
                </div>
            </div>
        `).join('');
    }

    async loadContainers() {
        const containers = [
            {
                id: 'abc123',
                name: 'web-app_nginx_1',
                agentId: 'agent-001',
                stack: 'web-app',
                state: 'running',
                image: 'nginx:latest',
                ports: ['80:80', '443:443']
            },
            {
                id: 'def456',
                name: 'web-app_app_1',
                agentId: 'agent-001',
                stack: 'web-app',
                state: 'running',
                image: 'myapp:v1.0',
                ports: ['3000:3000']
            }
        ];

        this.renderContainers(containers);
        document.getElementById('stat-containers').textContent = containers.length;
    }

    renderContainers(containers) {
        const container = document.getElementById('containers-list');
        
        if (containers.length === 0) {
            container.innerHTML = '<div class="col-12"><div class="alert alert-info">No containers found</div></div>';
            return;
        }

        container.innerHTML = containers.map(c => `
            <div class="col-md-6 col-lg-4 mb-3">
                <div class="card">
                    <div class="card-header d-flex justify-content-between align-items-center">
                        <h6 class="mb-0">${c.name}</h6>
                        <span class="badge bg-${c.state === 'running' ? 'success' : 'secondary'}">${c.state}</span>
                    </div>
                    <div class="card-body">
                        <p class="mb-1"><strong>ID:</strong> ${c.id.substring(0, 12)}</p>
                        <p class="mb-1"><strong>Stack:</strong> ${c.stack}</p>
                        <p class="mb-1"><strong>Image:</strong> ${c.image}</p>
                        <p class="mb-2"><strong>Ports:</strong> ${c.ports.join(', ')}</p>
                    </div>
                    <div class="card-footer">
                        <button class="btn btn-sm btn-outline-primary me-1" onclick="dashboard.viewContainer('${c.id}')">
                            <i class="fas fa-eye"></i> View
                        </button>
                        <button class="btn btn-sm btn-outline-warning me-1" onclick="dashboard.execContainer('${c.id}')">
                            <i class="fas fa-terminal"></i> Exec
                        </button>
                        <button class="btn btn-sm btn-outline-danger" onclick="dashboard.stopContainer('${c.id}')">
                            <i class="fas fa-stop"></i> Stop
                        </button>
                    </div>
                </div>
            </div>
        `).join('');
    }

    async loadOperations() {
        const operations = [
            {
                id: 'op-1',
                type: 'stack.apply',
                state: 'completed',
                progress: 100,
                createdAt: new Date(Date.now() - 3600000).toISOString(),
                completedAt: new Date(Date.now() - 3500000).toISOString()
            },
            {
                id: 'op-2',
                type: 'container.exec',
                state: 'running',
                progress: 45,
                createdAt: new Date(Date.now() - 60000).toISOString()
            }
        ];

        this.renderOperations(operations);
        document.getElementById('stat-operations').textContent = operations.length;
        document.getElementById('stat-operations-pending').textContent = 
            `${operations.filter(o => o.state === 'pending').length} pending`;
    }

    renderOperations(operations) {
        const container = document.getElementById('operations-list');
        
        if (operations.length === 0) {
            container.innerHTML = '<div class="col-12"><div class="alert alert-info">No operations found</div></div>';
            return;
        }

        container.innerHTML = operations.map(op => `
            <div class="col-md-6 col-lg-4 mb-3">
                <div class="card operation-card ${op.state}">
                    <div class="card-header d-flex justify-content-between align-items-center">
                        <h6 class="mb-0">${op.type}</h6>
                        <span class="badge bg-${this.getOperationStateColor(op.state)}">${op.state}</span>
                    </div>
                    <div class="card-body">
                        <p class="mb-1"><strong>ID:</strong> ${op.id}</p>
                        <p class="mb-1"><strong>Progress:</strong> ${op.progress}%</p>
                        <div class="progress mb-2">
                            <div class="progress-bar" role="progressbar" style="width: ${op.progress}%" aria-valuenow="${op.progress}" aria-valuemin="0" aria-valuemax="100"></div>
                        </div>
                        <p class="mb-1"><strong>Created:</strong> ${this.formatTime(op.createdAt)}</p>
                        ${op.completedAt ? `<p class="mb-0"><strong>Completed:</strong> ${this.formatTime(op.completedAt)}</p>` : ''}
                    </div>
                </div>
            </div>
        `).join('');
    }

    async loadLogs() {
        const logs = [
            { level: 'info', message: 'Agent agent-001 registered', timestamp: new Date(Date.now() - 10000).toISOString() },
            { level: 'info', message: 'Stack web-app applied successfully', timestamp: new Date(Date.now() - 20000).toISOString() },
            { level: 'warning', message: 'Agent agent-002 heartbeat delayed', timestamp: new Date(Date.now() - 30000).toISOString() },
            { level: 'error', message: 'Failed to apply stack api-service: timeout', timestamp: new Date(Date.now() - 40000).toISOString() }
        ];

        this.renderLogs(logs);
    }

    renderLogs(logs) {
        const container = document.getElementById('logs-container');
        container.innerHTML = logs.map(log => `
            <div class="log-entry ${log.level}">
                <span class="timestamp">[${this.formatTime(log.timestamp)}]</span>
                <span class="level">${log.level.toUpperCase()}</span>: ${log.message}
            </div>
        `).join('');
    }

    getOperationStateColor(state) {
        switch (state) {
            case 'pending': return 'warning';
            case 'running': return 'primary';
            case 'completed': return 'success';
            case 'failed': return 'danger';
            case 'cancelled': return 'secondary';
            default: return 'secondary';
        }
    }

    formatTime(isoString) {
        const date = new Date(isoString);
        const now = new Date();
        const diff = now - date;

        if (diff < 60000) return 'Just now';
        if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
        if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
        
        return date.toLocaleString();
    }

    showToast(message, type = 'info') {
        const container = document.getElementById('toast-container');
        const toastEl = document.createElement('div');
        toastEl.className = `toast align-items-center text-white bg-${type === 'success' ? 'success' : type === 'error' ? 'danger' : 'warning'} border-0`;
        toastEl.setAttribute('role', 'alert');
        toastEl.setAttribute('aria-live', 'assertive');
        toastEl.setAttribute('aria-atomic', 'true');
        
        toastEl.innerHTML = `
            <div class="d-flex">
                <div class="toast-body">${message}</div>
                <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button>
            </div>
        `;
        
        container.appendChild(toastEl);
        const toast = new bootstrap.Toast(toastEl, { delay: 3000 });
        toast.show();
        
        toastEl.addEventListener('hidden.bs.toast', () => toastEl.remove());
    }

    // Placeholder methods for future implementation
    viewAgent(id) {
        this.showToast(`Viewing agent: ${id}`, 'info');
    }

    removeAgent(id) {
        this.showToast(`Removing agent: ${id}`, 'warning');
    }

    viewStack(id) {
        this.showToast(`Viewing stack: ${id}`, 'info');
    }

    removeStack(id) {
        this.showToast(`Removing stack: ${id}`, 'warning');
    }

    viewContainer(id) {
        this.showToast(`Viewing container: ${id}`, 'info');
    }

    execContainer(id) {
        this.showToast(`Exec into container: ${id}`, 'info');
    }

    stopContainer(id) {
        this.showToast(`Stopping container: ${id}`, 'warning');
    }
}

// Initialize dashboard
const dashboard = new MandauDashboard();
