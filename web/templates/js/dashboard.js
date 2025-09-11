/**
 * Engine API Workflow - Dashboard JavaScript
 * Main JavaScript file for dashboard functionality
 */

// Global configuration
const config = {
    apiBaseUrl: '/api/v1',
    toastDuration: 5000,
    animationDuration: 300,
    dashboardRefreshInterval: 30000, // 30 seconds
    authTokenKey: 'auth_token'
};

// Authentication manager
const authManager = {
    getToken() {
        // Try to get token from cookie first, then localStorage
        const cookies = document.cookie.split(';');
        for (let cookie of cookies) {
            const [name, value] = cookie.trim().split('=');
            if (name === config.authTokenKey) {
                return value;
            }
        }
        return localStorage.getItem(config.authTokenKey);
    },

    setToken(token) {
        localStorage.setItem(config.authTokenKey, token);
    },

    removeToken() {
        localStorage.removeItem(config.authTokenKey);
        // Also remove from cookie
        document.cookie = `${config.authTokenKey}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
    },

    isAuthenticated() {
        return !!this.getToken();
    },

    logout() {
        this.removeToken();
        window.location.href = '/login';
    }
};

// Utility functions
const utils = {
    // Show notification toast
    showToast(message, type = 'info', duration = config.toastDuration) {
        const toastContainer = this.getToastContainer();
        const toast = this.createToast(message, type);
        
        toastContainer.appendChild(toast);
        
        // Trigger animation
        setTimeout(() => toast.classList.add('show'), 100);
        
        // Auto-remove
        setTimeout(() => {
            toast.classList.remove('show');
            setTimeout(() => toast.remove(), config.animationDuration);
        }, duration);
    },

    // Create toast container if it doesn't exist
    getToastContainer() {
        let container = document.getElementById('toast-container');
        if (!container) {
            container = document.createElement('div');
            container.id = 'toast-container';
            container.className = 'position-fixed top-0 end-0 p-3';
            container.style.zIndex = '9999';
            document.body.appendChild(container);
        }
        return container;
    },

    // Create toast element
    createToast(message, type) {
        const toastId = 'toast-' + Date.now();
        const iconMap = {
            success: 'bi-check-circle-fill',
            error: 'bi-x-circle-fill',
            warning: 'bi-exclamation-triangle-fill',
            info: 'bi-info-circle-fill'
        };

        const toast = document.createElement('div');
        toast.className = `toast align-items-center text-white bg-${type === 'error' ? 'danger' : type} border-0`;
        toast.setAttribute('role', 'alert');
        toast.id = toastId;
        
        toast.innerHTML = `
            <div class="d-flex">
                <div class="toast-body">
                    <i class="bi ${iconMap[type] || iconMap.info} me-2"></i>
                    ${message}
                </div>
                <button type="button" class="btn-close btn-close-white me-2 m-auto" onclick="this.closest('.toast').remove()"></button>
            </div>
        `;
        
        return toast;
    },

    // Format date
    formatDate(dateString) {
        const date = new Date(dateString);
        return date.toLocaleDateString('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
    },

    // Format duration in milliseconds to human readable
    formatDuration(ms) {
        if (ms < 1000) {
            return `${ms}ms`;
        }
        const seconds = ms / 1000;
        if (seconds < 60) {
            return `${seconds.toFixed(1)}s`;
        } else if (seconds < 3600) {
            return `${(seconds / 60).toFixed(1)}m`;
        } else {
            return `${(seconds / 3600).toFixed(1)}h`;
        }
    },

    // Format numbers with commas
    formatNumber(num) {
        return new Intl.NumberFormat().format(num);
    },

    // Format percentage
    formatPercentage(num) {
        return `${num.toFixed(1)}%`;
    },

    // Debounce function
    debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    },

    // Show loading spinner
    showLoading(element, text = 'Loading...') {
        element.innerHTML = `
            <div class="d-flex align-items-center">
                <div class="spinner-border spinner-border-sm me-2" role="status">
                    <span class="visually-hidden">Loading...</span>
                </div>
                ${text}
            </div>
        `;
        element.disabled = true;
    },

    // Hide loading spinner
    hideLoading(element, originalText) {
        element.innerHTML = originalText;
        element.disabled = false;
    },

    // Show page loading overlay
    showPageLoading() {
        let overlay = document.getElementById('page-loading-overlay');
        if (!overlay) {
            overlay = document.createElement('div');
            overlay.id = 'page-loading-overlay';
            overlay.className = 'position-fixed top-0 start-0 w-100 h-100 d-flex align-items-center justify-content-center';
            overlay.style.backgroundColor = 'rgba(255, 255, 255, 0.8)';
            overlay.style.zIndex = '9998';
            overlay.innerHTML = `
                <div class="text-center">
                    <div class="spinner-border text-primary" role="status">
                        <span class="visually-hidden">Loading...</span>
                    </div>
                    <div class="mt-2">Loading dashboard...</div>
                </div>
            `;
            document.body.appendChild(overlay);
        }
        overlay.style.display = 'flex';
    },

    // Hide page loading overlay
    hidePageLoading() {
        const overlay = document.getElementById('page-loading-overlay');
        if (overlay) {
            overlay.style.display = 'none';
        }
    }
};

// API helper functions
const api = {
    // Generic API request with authentication
    async request(endpoint, options = {}) {
        const url = config.apiBaseUrl + endpoint;
        const token = authManager.getToken();
        
        const defaultOptions = {
            headers: {
                'Content-Type': 'application/json',
            },
        };

        // Add authentication header if token exists
        if (token) {
            defaultOptions.headers['Authorization'] = `Bearer ${token}`;
        }

        try {
            const response = await fetch(url, { ...defaultOptions, ...options });
            
            // Handle authentication errors
            if (response.status === 401) {
                authManager.logout();
                return;
            }
            
            const data = await response.json();
            
            if (!response.ok) {
                throw new Error(data.message || `HTTP error! status: ${response.status}`);
            }
            
            return data;
        } catch (error) {
            console.error('API request failed:', error);
            throw error;
        }
    },

    // Dashboard API methods
    dashboard: {
        async getComplete(filters = {}) {
            const params = new URLSearchParams(filters);
            return api.request(`/dashboard?${params}`);
        },

        async getSummary() {
            return api.request('/dashboard/summary');
        },

        async getHealth() {
            return api.request('/dashboard/health');
        },

        async getAlerts() {
            return api.request('/dashboard/alerts');
        },

        async getMetrics(metrics, timeRange = '24h') {
            const params = new URLSearchParams({ 
                metrics: Array.isArray(metrics) ? metrics.join(',') : metrics,
                time_range: timeRange 
            });
            return api.request(`/dashboard/metrics?${params}`);
        },

        async refresh() {
            return api.request('/dashboard/refresh', { method: 'POST' });
        }
    },

    // Workflow API methods
    workflows: {
        async list(page = 1, search = '', status = '') {
            const params = new URLSearchParams({ page, search, status });
            return api.request(`/workflows?${params}`);
        },

        async get(id) {
            return api.request(`/workflows/${id}`);
        },

        async create(data) {
            return api.request('/workflows', {
                method: 'POST',
                body: JSON.stringify(data)
            });
        },

        async update(id, data) {
            return api.request(`/workflows/${id}`, {
                method: 'PUT',
                body: JSON.stringify(data)
            });
        },

        async delete(id) {
            return api.request(`/workflows/${id}`, {
                method: 'DELETE'
            });
        },

        async toggle(id) {
            return api.request(`/workflows/${id}/toggle`, {
                method: 'POST'
            });
        },

        async clone(id, name, description = '') {
            return api.request(`/workflows/${id}/clone`, {
                method: 'POST',
                body: JSON.stringify({ name, description })
            });
        },

        async trigger(id, data = {}) {
            return api.request('/triggers/workflow', {
                method: 'POST',
                body: JSON.stringify({
                    workflow_id: id,
                    trigger_by: 'manual',
                    data
                })
            });
        }
    },

    // Logs API methods
    logs: {
        async list(page = 1, workflowId = '', status = '') {
            const params = new URLSearchParams({ page, workflow_id: workflowId, status });
            return api.request(`/logs?${params}`);
        },

        async get(id) {
            return api.request(`/logs/${id}`);
        }
    }
};

// Dashboard data manager
const dashboardManager = {
    data: null,
    lastUpdate: null,
    updateInterval: null,

    async init() {
        if (!this.isDashboardPage()) return;

        try {
            utils.showPageLoading();
            await this.loadDashboardData();
            this.renderDashboard();
            this.startAutoRefresh();
        } catch (error) {
            console.error('Failed to initialize dashboard:', error);
            utils.showToast('Failed to load dashboard data', 'error');
        } finally {
            utils.hidePageLoading();
        }
    },

    isDashboardPage() {
        return document.querySelector('[data-page="dashboard"]') || 
               window.location.pathname === '/' || 
               window.location.pathname === '/dashboard';
    },

    async loadDashboardData() {
        try {
            const [dashboardData, summary, health, alerts] = await Promise.all([
                api.dashboard.getComplete(),
                api.dashboard.getSummary(),
                api.dashboard.getHealth(),
                api.dashboard.getAlerts()
            ]);

            this.data = {
                dashboard: dashboardData.data,
                summary: summary.data,
                health: health.data,
                alerts: alerts.data
            };

            this.lastUpdate = new Date();
        } catch (error) {
            console.error('Failed to load dashboard data:', error);
            throw error;
        }
    },

    renderDashboard() {
        if (!this.data) return;

        this.renderQuickStats();
        this.renderSystemHealth();
        this.renderRecentActivity();
        this.renderWorkflowStatus();
        this.renderAlerts();
        this.updateLastRefreshTime();
    },

    renderQuickStats() {
        const stats = this.data.dashboard?.quick_stats || this.data.summary;
        if (!stats) return;

        const statElements = {
            'active-workflows': stats.active_workflows || stats.activeWorkflows,
            'total-executions': stats.total_executions || stats.totalExecutions,
            'success-rate': stats.success_rate_24h || stats.successRate,
            'executions-today': stats.executions_today || stats.executionsToday,
            'avg-execution-time': stats.avg_execution_time || stats.averageExecutionTime,
            'queue-length': stats.queue_length || stats.currentQueueLength,
            'total-users': stats.total_users || stats.totalUsers,
            'errors-last-24h': stats.errors_last_24h || 0
        };

        Object.entries(statElements).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element && value !== undefined) {
                let formattedValue = value;
                
                // Format based on the stat type
                if (id.includes('rate')) {
                    formattedValue = utils.formatPercentage(value);
                } else if (id.includes('time')) {
                    formattedValue = utils.formatDuration(value);
                } else if (typeof value === 'number' && value > 999) {
                    formattedValue = utils.formatNumber(value);
                }
                
                element.textContent = formattedValue;
                element.classList.add('pulse');
                setTimeout(() => element.classList.remove('pulse'), 1000);
            }
        });
    },

    renderSystemHealth() {
        const health = this.data.health;
        if (!health) return;

        const healthElement = document.getElementById('system-health-status');
        const uptimeElement = document.getElementById('system-uptime');
        const versionElement = document.getElementById('system-version');

        if (healthElement) {
            healthElement.textContent = health.status;
            healthElement.className = `badge bg-${this.getHealthColor(health.status)}`;
        }

        if (uptimeElement) {
            uptimeElement.textContent = health.uptime || 'N/A';
        }

        if (versionElement) {
            versionElement.textContent = health.version || '1.0.0';
        }

        // Update health indicators
        const indicators = document.querySelectorAll('.health-indicator');
        indicators.forEach(indicator => {
            const metric = indicator.dataset.metric;
            if (health[metric] !== undefined) {
                const value = health[metric];
                const isHealthy = this.isMetricHealthy(metric, value);
                indicator.className = `health-indicator ${isHealthy ? 'text-success' : 'text-danger'}`;
                indicator.querySelector('.health-value').textContent = this.formatHealthMetric(metric, value);
            }
        });
    },

    renderRecentActivity() {
        const activity = this.data.dashboard?.recent_activity;
        if (!activity?.length) return;

        const container = document.querySelector('.recent-activity .timeline');
        if (!container) return;

        container.innerHTML = activity.slice(0, 10).map(item => `
            <div class="timeline-item fade-in">
                <div class="timeline-marker">
                    ${this.getActivityIcon(item.type, item.status)}
                </div>
                <div class="timeline-content">
                    <div class="fw-bold">${item.workflow_name || item.message}</div>
                    <div class="text-muted small">
                        ${item.user_name ? `by ${item.user_name} • ` : ''}
                        ${utils.formatDate(item.timestamp)}
                    </div>
                    ${item.duration ? `<div class="text-info small">Duration: ${utils.formatDuration(item.duration)}</div>` : ''}
                </div>
            </div>
        `).join('');
    },

    renderWorkflowStatus() {
        const workflows = this.data.dashboard?.workflow_status;
        if (!workflows?.length) return;

        const container = document.querySelector('.workflow-status-list');
        if (!container) return;

        container.innerHTML = workflows.slice(0, 5).map(workflow => `
            <div class="list-group-item">
                <div class="d-flex justify-content-between align-items-center">
                    <div>
                        <h6 class="mb-1">${workflow.name}</h6>
                        <small class="text-muted">
                            Success Rate: ${utils.formatPercentage(workflow.success_rate)} • 
                            Runs: ${workflow.total_runs}
                        </small>
                    </div>
                    <div class="text-end">
                        <span class="badge bg-${this.getWorkflowStatusColor(workflow.status)}">${workflow.status}</span>
                        ${workflow.last_execution ? `<br><small class="text-muted">${utils.formatDate(workflow.last_execution)}</small>` : ''}
                    </div>
                </div>
            </div>
        `).join('');
    },

    renderAlerts() {
        const alerts = this.data.alerts;
        if (!alerts?.length) return;

        const container = document.querySelector('.alerts-container');
        if (!container) return;

        const activeAlerts = alerts.filter(alert => alert.is_active);
        
        if (activeAlerts.length === 0) {
            container.innerHTML = '<div class="alert alert-success">No active alerts</div>';
            return;
        }

        container.innerHTML = activeAlerts.slice(0, 5).map(alert => `
            <div class="alert alert-${this.getAlertColor(alert.severity)} alert-dismissible">
                <strong>${alert.title}</strong>
                <p class="mb-1">${alert.message}</p>
                <small class="text-muted">${utils.formatDate(alert.timestamp)}</small>
                <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
            </div>
        `).join('');
    },

    updateLastRefreshTime() {
        const element = document.getElementById('last-refresh-time');
        if (element && this.lastUpdate) {
            element.textContent = utils.formatDate(this.lastUpdate);
        }
    },

    startAutoRefresh() {
        this.updateInterval = setInterval(async () => {
            try {
                await this.loadDashboardData();
                this.renderDashboard();
            } catch (error) {
                console.warn('Auto-refresh failed:', error);
            }
        }, config.dashboardRefreshInterval);

        // Stop refresh when page is hidden
        document.addEventListener('visibilitychange', () => {
            if (document.hidden) {
                this.stopAutoRefresh();
            } else {
                this.startAutoRefresh();
            }
        });
    },

    stopAutoRefresh() {
        if (this.updateInterval) {
            clearInterval(this.updateInterval);
            this.updateInterval = null;
        }
    },

    // Helper methods
    getHealthColor(status) {
        const colors = {
            'healthy': 'success',
            'warning': 'warning',
            'critical': 'danger'
        };
        return colors[status] || 'secondary';
    },

    getWorkflowStatusColor(status) {
        const colors = {
            'active': 'success',
            'inactive': 'secondary',
            'error': 'danger',
            'warning': 'warning'
        };
        return colors[status] || 'secondary';
    },

    getAlertColor(severity) {
        const colors = {
            'critical': 'danger',
            'warning': 'warning',
            'info': 'info'
        };
        return colors[severity] || 'info';
    },

    getActivityIcon(type, status) {
        if (type === 'execution') {
            const icons = {
                'success': '<i class="bi bi-check-circle-fill text-success"></i>',
                'failed': '<i class="bi bi-x-circle-fill text-danger"></i>',
                'running': '<i class="bi bi-arrow-clockwise text-primary"></i>',
                'pending': '<i class="bi bi-clock text-warning"></i>'
            };
            return icons[status] || icons.pending;
        }
        
        const typeIcons = {
            'error': '<i class="bi bi-exclamation-triangle-fill text-danger"></i>',
            'warning': '<i class="bi bi-exclamation-triangle-fill text-warning"></i>',
            'info': '<i class="bi bi-info-circle-fill text-info"></i>',
            'workflow_created': '<i class="bi bi-plus-circle-fill text-success"></i>'
        };
        return typeIcons[type] || typeIcons.info;
    },

    isMetricHealthy(metric, value) {
        const thresholds = {
            'cpu_usage': 80,
            'memory_usage': 85,
            'api_response_time': 1000,
            'db_connections': 100
        };
        
        if (metric === 'redis_connected') return value === true;
        
        const threshold = thresholds[metric];
        return threshold ? value < threshold : true;
    },

    formatHealthMetric(metric, value) {
        if (metric === 'redis_connected') {
            return value ? 'Connected' : 'Disconnected';
        }
        
        if (metric.includes('usage') || metric.includes('cpu')) {
            return utils.formatPercentage(value);
        }
        
        if (metric.includes('time')) {
            return utils.formatDuration(value);
        }
        
        return value.toString();
    }
};

// Workflow management functions (preserved from original)
const workflowManager = {
    async triggerWorkflow(workflowId, buttonElement = null) {
        if (!confirm('Are you sure you want to run this workflow?')) return;

        const originalText = buttonElement ? buttonElement.innerHTML : null;
        
        try {
            if (buttonElement) {
                utils.showLoading(buttonElement, 'Running...');
            }

            await api.workflows.trigger(workflowId);
            utils.showToast('Workflow triggered successfully!', 'success');
            
            // Refresh the page after a short delay
            setTimeout(() => location.reload(), 2000);
            
        } catch (error) {
            utils.showToast('Failed to trigger workflow: ' + error.message, 'error');
        } finally {
            if (buttonElement && originalText) {
                utils.hideLoading(buttonElement, originalText);
            }
        }
    },

    async toggleWorkflow(workflowId, currentStatus, buttonElement = null) {
        const action = currentStatus === 'active' ? 'deactivate' : 'activate';
        if (!confirm(`Are you sure you want to ${action} this workflow?`)) return;

        const originalText = buttonElement ? buttonElement.innerHTML : null;

        try {
            if (buttonElement) {
                utils.showLoading(buttonElement, `${action.charAt(0).toUpperCase() + action.slice(1)}ing...`);
            }

            await api.workflows.toggle(workflowId);
            utils.showToast(`Workflow ${action}d successfully!`, 'success');
            
            setTimeout(() => location.reload(), 1500);
            
        } catch (error) {
            utils.showToast(`Failed to ${action} workflow: ` + error.message, 'error');
        } finally {
            if (buttonElement && originalText) {
                utils.hideLoading(buttonElement, originalText);
            }
        }
    },

    async cloneWorkflow(workflowId, buttonElement = null) {
        const name = prompt('Enter a name for the cloned workflow:');
        if (!name) return;

        const originalText = buttonElement ? buttonElement.innerHTML : null;

        try {
            if (buttonElement) {
                utils.showLoading(buttonElement, 'Cloning...');
            }

            await api.workflows.clone(workflowId, name);
            utils.showToast('Workflow cloned successfully!', 'success');
            
            setTimeout(() => location.reload(), 1500);
            
        } catch (error) {
            utils.showToast('Failed to clone workflow: ' + error.message, 'error');
        } finally {
            if (buttonElement && originalText) {
                utils.hideLoading(buttonElement, originalText);
            }
        }
    },

    async deleteWorkflow(workflowId, workflowName, buttonElement = null) {
        if (!confirm(`Are you sure you want to delete "${workflowName}"? This action cannot be undone.`)) return;

        const originalText = buttonElement ? buttonElement.innerHTML : null;

        try {
            if (buttonElement) {
                utils.showLoading(buttonElement, 'Deleting...');
            }

            await api.workflows.delete(workflowId);
            utils.showToast('Workflow deleted successfully!', 'success');
            
            setTimeout(() => location.reload(), 1500);
            
        } catch (error) {
            utils.showToast('Failed to delete workflow: ' + error.message, 'error');
        } finally {
            if (buttonElement && originalText) {
                utils.hideLoading(buttonElement, originalText);
            }
        }
    }
};

// Search and filter functionality (preserved from original)
const searchManager = {
    init() {
        this.setupSearchInputs();
        this.setupFilters();
    },

    setupSearchInputs() {
        const searchInputs = document.querySelectorAll('input[name="search"]');
        searchInputs.forEach(input => {
            input.addEventListener('input', utils.debounce((e) => {
                this.performSearch(e.target.value);
            }, 500));
        });
    },

    setupFilters() {
        const filterSelects = document.querySelectorAll('select[name="status"], select[name="workflow_id"]');
        filterSelects.forEach(select => {
            select.addEventListener('change', (e) => {
                this.applyFilter(e.target.name, e.target.value);
            });
        });
    },

    performSearch(query) {
        const url = new URL(window.location);
        if (query) {
            url.searchParams.set('search', query);
        } else {
            url.searchParams.delete('search');
        }
        url.searchParams.set('page', '1');
        window.location.href = url.toString();
    },

    applyFilter(filterName, value) {
        const url = new URL(window.location);
        if (value) {
            url.searchParams.set(filterName, value);
        } else {
            url.searchParams.delete(filterName);
        }
        url.searchParams.set('page', '1');
        window.location.href = url.toString();
    }
};

// Form validation and enhancement (preserved from original)
const formManager = {
    init() {
        this.setupFormValidation();
        this.setupFormSubmission();
    },

    setupFormValidation() {
        const forms = document.querySelectorAll('form[data-validate]');
        forms.forEach(form => {
            form.addEventListener('submit', (e) => {
                if (!this.validateForm(form)) {
                    e.preventDefault();
                }
            });
        });
    },

    setupFormSubmission() {
        const forms = document.querySelectorAll('form[data-ajax]');
        forms.forEach(form => {
            form.addEventListener('submit', (e) => {
                e.preventDefault();
                this.submitFormAjax(form);
            });
        });
    },

    validateForm(form) {
        let isValid = true;
        const inputs = form.querySelectorAll('input[required], select[required], textarea[required]');
        
        inputs.forEach(input => {
            if (!input.value.trim()) {
                this.showFieldError(input, 'This field is required');
                isValid = false;
            } else {
                this.clearFieldError(input);
            }
        });

        return isValid;
    },

    showFieldError(input, message) {
        input.classList.add('is-invalid');
        
        let feedback = input.nextElementSibling;
        if (!feedback || !feedback.classList.contains('invalid-feedback')) {
            feedback = document.createElement('div');
            feedback.className = 'invalid-feedback';
            input.parentNode.insertBefore(feedback, input.nextSibling);
        }
        feedback.textContent = message;
    },

    clearFieldError(input) {
        input.classList.remove('is-invalid');
        const feedback = input.nextElementSibling;
        if (feedback && feedback.classList.contains('invalid-feedback')) {
            feedback.remove();
        }
    },

    async submitFormAjax(form) {
        const submitBtn = form.querySelector('button[type="submit"]');
        const originalText = submitBtn.innerHTML;
        
        try {
            utils.showLoading(submitBtn, 'Submitting...');
            
            const formData = new FormData(form);
            const data = Object.fromEntries(formData);
            
            const response = await fetch(form.action, {
                method: form.method || 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            const result = await response.json();
            
            if (response.ok) {
                utils.showToast('Form submitted successfully!', 'success');
                if (form.dataset.redirect) {
                    setTimeout(() => window.location.href = form.dataset.redirect, 1500);
                }
            } else {
                utils.showToast(result.message || 'Form submission failed', 'error');
            }
            
        } catch (error) {
            utils.showToast('Form submission failed: ' + error.message, 'error');
        } finally {
            utils.hideLoading(submitBtn, originalText);
        }
    }
};

// Initialize everything when DOM is loaded
document.addEventListener('DOMContentLoaded', function() {
    // Initialize managers
    searchManager.init();
    formManager.init();
    
    // Initialize dashboard if on dashboard page
    dashboardManager.init();
    
    // Setup global event listeners
    setupGlobalEventListeners();
    
    // Setup tooltips
    setupTooltips();
    
    console.log('Dashboard initialized');
});

// Global event listeners
function setupGlobalEventListeners() {
    // Handle workflow actions
    document.addEventListener('click', function(e) {
        if (e.target.matches('[data-action="trigger-workflow"]')) {
            e.preventDefault();
            const workflowId = e.target.dataset.workflowId;
            workflowManager.triggerWorkflow(workflowId, e.target);
        }
        
        if (e.target.matches('[data-action="toggle-workflow"]')) {
            e.preventDefault();
            const workflowId = e.target.dataset.workflowId;
            const status = e.target.dataset.status;
            workflowManager.toggleWorkflow(workflowId, status, e.target);
        }
        
        if (e.target.matches('[data-action="clone-workflow"]')) {
            e.preventDefault();
            const workflowId = e.target.dataset.workflowId;
            workflowManager.cloneWorkflow(workflowId, e.target);
        }
        
        if (e.target.matches('[data-action="delete-workflow"]')) {
            e.preventDefault();
            const workflowId = e.target.dataset.workflowId;
            const workflowName = e.target.dataset.workflowName;
            workflowManager.deleteWorkflow(workflowId, workflowName, e.target);
        }
        
        // Handle dashboard refresh button
        if (e.target.matches('[data-action="refresh-dashboard"]')) {
            e.preventDefault();
            dashboardManager.refresh();
        }
    });

    // Handle keyboard shortcuts
    document.addEventListener('keydown', function(e) {
        // Ctrl/Cmd + K to focus search
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            const searchInput = document.querySelector('input[name="search"]');
            if (searchInput) {
                searchInput.focus();
                searchInput.select();
            }
        }
        
        // Escape to clear search
        if (e.key === 'Escape') {
            const searchInput = document.querySelector('input[name="search"]:focus');
            if (searchInput) {
                searchInput.value = '';
                searchInput.blur();
            }
        }
        
        // F5 or Ctrl+R to refresh dashboard
        if (e.key === 'F5' || (e.ctrlKey && e.key === 'r')) {
            if (dashboardManager.isDashboardPage()) {
                e.preventDefault();
                dashboardManager.refresh();
            }
        }
    });
}

// Dashboard refresh method
dashboardManager.refresh = async function() {
    try {
        utils.showPageLoading();
        await this.loadDashboardData();
        this.renderDashboard();
        utils.showToast('Dashboard refreshed successfully', 'success');
    } catch (error) {
        console.error('Failed to refresh dashboard:', error);
        utils.showToast('Failed to refresh dashboard', 'error');
    } finally {
        utils.hidePageLoading();
    }
};

// Setup Bootstrap tooltips
function setupTooltips() {
    // Initialize Bootstrap tooltips if available
    if (typeof bootstrap !== 'undefined' && bootstrap.Tooltip) {
        const tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl) {
            return new bootstrap.Tooltip(tooltipTriggerEl);
        });
    }
}

// Export functions to global scope for backward compatibility
window.triggerWorkflow = workflowManager.triggerWorkflow.bind(workflowManager);
window.toggleWorkflow = workflowManager.toggleWorkflow.bind(workflowManager);
window.cloneWorkflow = workflowManager.cloneWorkflow.bind(workflowManager);
window.deleteWorkflow = workflowManager.deleteWorkflow.bind(workflowManager);
window.showAlert = utils.showToast.bind(utils);
window.refreshDashboard = dashboardManager.refresh.bind(dashboardManager);

// Utility functions for templates
window.seq = function(start, end) {
    const result = [];
    for (let i = start; i <= end; i++) {
        result.push(i);
    }
    return result;
};

window.sub = (a, b) => a - b;
window.add = (a, b) => a + b;
window.mul = (a, b) => a * b;
window.div = (a, b) => b !== 0 ? a / b : 0;

// Error boundary for uncaught errors
window.addEventListener('error', function(e) {
    console.error('Global error:', e.error);
    utils.showToast('An unexpected error occurred', 'error');
});

// Handle promise rejections
window.addEventListener('unhandledrejection', function(e) {
    console.error('Unhandled promise rejection:', e.reason);
    utils.showToast('An unexpected error occurred', 'error');
    e.preventDefault();
});

// Development helpers (only in development)
if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
    window.dashboardDebug = {
        api,
        dashboardManager,
        utils,
        authManager
    };
    console.log('Dashboard debug tools available at window.dashboardDebug');
}