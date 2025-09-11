/**
 * Engine API Workflow - Dashboard JavaScript
 * Main JavaScript file for dashboard functionality
 */

// Global configuration
const config = {
    apiBaseUrl: '/api/v1',
    toastDuration: 5000,
    animationDuration: 300
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

    // Format duration in seconds to human readable
    formatDuration(seconds) {
        if (seconds < 60) {
            return `${seconds.toFixed(1)}s`;
        } else if (seconds < 3600) {
            return `${(seconds / 60).toFixed(1)}m`;
        } else {
            return `${(seconds / 3600).toFixed(1)}h`;
        }
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
    }
};

// API helper functions
const api = {
    // Generic API request
    async request(endpoint, options = {}) {
        const url = config.apiBaseUrl + endpoint;
        const defaultOptions = {
            headers: {
                'Content-Type': 'application/json',
            },
        };

        try {
            const response = await fetch(url, { ...defaultOptions, ...options });
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

// Workflow management functions
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

// Search and filter functionality
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
        url.searchParams.set('page', '1'); // Reset to first page
        window.location.href = url.toString();
    },

    applyFilter(filterName, value) {
        const url = new URL(window.location);
        if (value) {
            url.searchParams.set(filterName, value);
        } else {
            url.searchParams.delete(filterName);
        }
        url.searchParams.set('page', '1'); // Reset to first page
        window.location.href = url.toString();
    }
};

// Dashboard real-time updates
const dashboardUpdates = {
    updateInterval: 30000, // 30 seconds
    intervalId: null,

    init() {
        this.startUpdates();
        this.setupVisibilityHandling();
    },

    startUpdates() {
        this.intervalId = setInterval(() => {
            this.updateStats();
            this.updateRecentActivity();
        }, this.updateInterval);
    },

    stopUpdates() {
        if (this.intervalId) {
            clearInterval(this.intervalId);
            this.intervalId = null;
        }
    },

    setupVisibilityHandling() {
        document.addEventListener('visibilitychange', () => {
            if (document.hidden) {
                this.stopUpdates();
            } else {
                this.startUpdates();
            }
        });
    },

    async updateStats() {
        try {
            // Only update if we're on the dashboard page
            if (!document.querySelector('.stats-cards')) return;

            // Update stats without full page reload
            const response = await fetch('/api/v1/dashboard/stats');
            const data = await response.json();
            
            if (data.success) {
                this.renderStats(data.data);
            }
        } catch (error) {
            console.warn('Failed to update stats:', error);
        }
    },

    async updateRecentActivity() {
        try {
            // Only update if we're on the dashboard page
            if (!document.querySelector('.recent-activity')) return;

            const response = await fetch('/api/v1/dashboard/activity');
            const data = await response.json();
            
            if (data.success) {
                this.renderActivity(data.data);
            }
        } catch (error) {
            console.warn('Failed to update activity:', error);
        }
    },

    renderStats(stats) {
        // Update stat cards with new data
        const statElements = {
            'total-executions': stats.totalExecutions,
            'success-rate': stats.successRate?.toFixed(1) + '%',
            'executions-today': stats.executionsToday,
            'avg-execution-time': stats.averageExecutionTime?.toFixed(2) + 's'
        };

        Object.entries(statElements).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element) {
                element.textContent = value;
                element.classList.add('pulse');
                setTimeout(() => element.classList.remove('pulse'), 1000);
            }
        });
    },

    renderActivity(activity) {
        const container = document.querySelector('.recent-activity .timeline');
        if (!container || !activity?.length) return;

        // Update recent activity timeline
        container.innerHTML = activity.map(log => `
            <div class="timeline-item fade-in">
                <div class="timeline-marker">
                    ${this.getStatusIcon(log.status)}
                </div>
                <div class="timeline-content">
                    <div class="fw-bold">${log.workflowName}</div>
                    <div class="text-muted small">
                        ${log.triggerType} trigger • ${utils.formatDate(log.createdAt)}
                    </div>
                    ${log.errorMessage ? `<div class="text-danger small mt-1">${log.errorMessage}</div>` : ''}
                </div>
            </div>
        `).join('');
    },

    getStatusIcon(status) {
        const icons = {
            completed: '<i class="bi bi-check-circle-fill text-success"></i>',
            failed: '<i class="bi bi-x-circle-fill text-danger"></i>',
            running: '<i class="bi bi-arrow-clockwise text-primary"></i>',
            pending: '<i class="bi bi-clock text-warning"></i>'
        };
        return icons[status] || icons.pending;
    }
};

// Form validation and enhancement
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
    
    // Initialize dashboard updates only on dashboard page
    if (document.querySelector('[data-page="dashboard"]')) {
        dashboardUpdates.init();
    }
    
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
    });
}

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

// Utility function for templates
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