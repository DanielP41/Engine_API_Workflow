// charts.js - Integración de gráficos con Chart.js para Engine API Workflow
// Este archivo extiende el dashboard.js existente con funcionalidades de gráficos

class ChartsManager {
    constructor() {
        this.charts = {};
        this.apiBase = '/api/v1';
        this.currentTimeRange = '24h';
        this.colors = {
            primary: '#667eea',
            secondary: '#764ba2',
            success: '#1cc88a',
            danger: '#e74a3b',
            warning: '#f6c23e',
            info: '#36b9cc'
        };
    }

    // Integración con la API existente del proyecto
    async fetchPerformanceData(timeRange = '24h') {
        try {
            const token = localStorage.getItem('access_token');
            const response = await fetch(`${this.apiBase}/dashboard/performance?time_range=${timeRange}`, {
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                }
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const result = await response.json();
            return result.data; // Usar estructura de tu API: { success: true, data: {...} }
        } catch (error) {
            console.error('Error fetching performance data:', error);
            return this.getFallbackData();
        }
    }

    async fetchDashboardSummary() {
        try {
            const token = localStorage.getItem('access_token');
            const response = await fetch(`${this.apiBase}/dashboard/summary`, {
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                }
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const result = await response.json();
            return result.data;
        } catch (error) {
            console.error('Error fetching dashboard summary:', error);
            return this.getFallbackSummary();
        }
    }

    // Datos de respaldo en caso de error de API
    getFallbackData() {
        return {
            executions_last_24h: [],
            executions_last_7d: [],
            success_rate_trend: [],
            avg_execution_time: [],
            queue_length_trend: [],
            error_distribution: [],
            trigger_distribution: [],
            workflow_distribution: [],
            user_activity_heatmap: [],
            system_resource_usage: [],
            hourly_executions: [],
            weekly_trends: []
        };
    }

    getFallbackSummary() {
        return {
            system_status: 'healthy',
            total_workflows: 0,
            active_workflows: 0,
            total_executions: 0,
            success_rate: 0,
            current_queue_length: 0,
            average_response_time: 0,
            last_update: new Date().toISOString(),
            critical_alerts: 0
        };
    }

    // Inicializar todos los gráficos
    async initializeCharts(containerId = 'dashboard-charts') {
        const container = document.getElementById(containerId);
        if (!container) {
            console.error(`Container ${containerId} not found`);
            return;
        }

        // Cargar datos de la API
        const [performanceData, summaryData] = await Promise.all([
            this.fetchPerformanceData(this.currentTimeRange),
            this.fetchDashboardSummary()
        ]);

        this.performanceData = performanceData;
        this.summaryData = summaryData;

        // Crear contenedores para gráficos si no existen
        this.createChartContainers(container);

        // Inicializar cada gráfico
        this.createExecutionTrendChart();
        this.createSuccessRateChart();
        this.createResponseTimeChart();
        this.createQueueLengthChart();
        this.createErrorDistributionChart();
        this.createWorkflowDistributionChart();
        this.createHourlyActivityChart();
        this.createSystemResourcesChart();

        console.log('Charts initialized successfully');
    }

    createChartContainers(container) {
        const chartsHTML = `
            <div class="row mb-4">
                <div class="col-xl-6 col-lg-6">
                    <div class="card">
                        <div class="card-header">
                            <h6 class="m-0 font-weight-bold text-primary">Tendencia de Ejecuciones</h6>
                        </div>
                        <div class="card-body">
                            <canvas id="executionTrendChart" height="300"></canvas>
                        </div>
                    </div>
                </div>
                <div class="col-xl-6 col-lg-6">
                    <div class="card">
                        <div class="card-header">
                            <h6 class="m-0 font-weight-bold text-primary">Tasa de Éxito</h6>
                        </div>
                        <div class="card-body">
                            <canvas id="successRateChart" height="300"></canvas>
                        </div>
                    </div>
                </div>
            </div>

            <div class="row mb-4">
                <div class="col-xl-6 col-lg-6">
                    <div class="card">
                        <div class="card-header">
                            <h6 class="m-0 font-weight-bold text-primary">Tiempo de Respuesta</h6>
                        </div>
                        <div class="card-body">
                            <canvas id="responseTimeChart" height="300"></canvas>
                        </div>
                    </div>
                </div>
                <div class="col-xl-6 col-lg-6">
                    <div class="card">
                        <div class="card-header">
                            <h6 class="m-0 font-weight-bold text-primary">Cola de Trabajos</h6>
                        </div>
                        <div class="card-body">
                            <canvas id="queueLengthChart" height="300"></canvas>
                        </div>
                    </div>
                </div>
            </div>

            <div class="row mb-4">
                <div class="col-xl-4 col-lg-6">
                    <div class="card">
                        <div class="card-header">
                            <h6 class="m-0 font-weight-bold text-primary">Distribución de Errores</h6>
                        </div>
                        <div class="card-body">
                            <canvas id="errorDistributionChart" height="300"></canvas>
                        </div>
                    </div>
                </div>
                <div class="col-xl-4 col-lg-6">
                    <div class="card">
                        <div class="card-header">
                            <h6 class="m-0 font-weight-bold text-primary">Workflows Más Usados</h6>
                        </div>
                        <div class="card-body">
                            <canvas id="workflowDistributionChart" height="300"></canvas>
                        </div>
                    </div>
                </div>
                <div class="col-xl-4 col-lg-12">
                    <div class="card">
                        <div class="card-header">
                            <h6 class="m-0 font-weight-bold text-primary">Recursos del Sistema</h6>
                        </div>
                        <div class="card-body">
                            <canvas id="systemResourcesChart" height="300"></canvas>
                        </div>
                    </div>
                </div>
            </div>

            <div class="row mb-4">
                <div class="col-12">
                    <div class="card">
                        <div class="card-header">
                            <h6 class="m-0 font-weight-bold text-primary">Actividad por Horas (Últimos 7 días)</h6>
                        </div>
                        <div class="card-body">
                            <canvas id="hourlyActivityChart" height="200"></canvas>
                        </div>
                    </div>
                </div>
            </div>
        `;

        container.innerHTML = chartsHTML;
    }

    createExecutionTrendChart() {
        const ctx = document.getElementById('executionTrendChart')?.getContext('2d');
        if (!ctx) return;

        const data = this.performanceData.executions_last_24h || [];
        
        this.charts.executionTrend = new Chart(ctx, {
            type: 'line',
            data: {
                labels: data.map(item => this.formatTimestamp(item.timestamp)),
                datasets: [{
                    label: 'Ejecuciones',
                    data: data.map(item => item.value || 0),
                    borderColor: this.colors.primary,
                    backgroundColor: this.hexToRgba(this.colors.primary, 0.1),
                    tension: 0.4,
                    fill: true
                }]
            },
            options: this.getBaseLineChartOptions()
        });
    }

    createSuccessRateChart() {
        const ctx = document.getElementById('successRateChart')?.getContext('2d');
        if (!ctx) return;

        const data = this.performanceData.success_rate_trend || [];

        this.charts.successRate = new Chart(ctx, {
            type: 'line',
            data: {
                labels: data.map(item => this.formatTimestamp(item.timestamp)),
                datasets: [{
                    label: 'Tasa de Éxito (%)',
                    data: data.map(item => item.value || 0),
                    borderColor: this.colors.success,
                    backgroundColor: this.hexToRgba(this.colors.success, 0.1),
                    tension: 0.4,
                    fill: true
                }]
            },
            options: {
                ...this.getBaseLineChartOptions(),
                scales: {
                    y: {
                        beginAtZero: false,
                        min: 90,
                        max: 100,
                        ticks: {
                            callback: function(value) {
                                return value + '%';
                            }
                        }
                    }
                }
            }
        });
    }

    createResponseTimeChart() {
        const ctx = document.getElementById('responseTimeChart')?.getContext('2d');
        if (!ctx) return;

        const data = this.performanceData.avg_execution_time || [];

        this.charts.responseTime = new Chart(ctx, {
            type: 'line',
            data: {
                labels: data.map(item => this.formatTimestamp(item.timestamp)),
                datasets: [{
                    label: 'Tiempo Promedio (ms)',
                    data: data.map(item => item.value || 0),
                    borderColor: this.colors.warning,
                    backgroundColor: this.hexToRgba(this.colors.warning, 0.1),
                    tension: 0.4,
                    fill: true
                }]
            },
            options: this.getBaseLineChartOptions()
        });
    }

    createQueueLengthChart() {
        const ctx = document.getElementById('queueLengthChart')?.getContext('2d');
        if (!ctx) return;

        const data = this.performanceData.queue_length_trend || [];

        this.charts.queueLength = new Chart(ctx, {
            type: 'area',
            data: {
                labels: data.map(item => this.formatTimestamp(item.timestamp)),
                datasets: [{
                    label: 'Elementos en Cola',
                    data: data.map(item => item.value || 0),
                    borderColor: this.colors.info,
                    backgroundColor: this.hexToRgba(this.colors.info, 0.3),
                    tension: 0.4,
                    fill: true
                }]
            },
            options: this.getBaseLineChartOptions()
        });
    }

    createErrorDistributionChart() {
        const ctx = document.getElementById('errorDistributionChart')?.getContext('2d');
        if (!ctx) return;

        const data = this.performanceData.error_distribution || [];

        this.charts.errorDistribution = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: data.map(item => item.error_type || item.type),
                datasets: [{
                    data: data.map(item => item.count || 0),
                    backgroundColor: [
                        this.colors.danger,
                        this.colors.warning,
                        this.colors.info,
                        this.colors.secondary
                    ]
                }]
            },
            options: this.getBaseDoughnutChartOptions()
        });
    }

    createWorkflowDistributionChart() {
        const ctx = document.getElementById('workflowDistributionChart')?.getContext('2d');
        if (!ctx) return;

        const data = this.performanceData.workflow_distribution || [];

        this.charts.workflowDistribution = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: data.map(item => item.workflow_name || item.name),
                datasets: [{
                    label: 'Ejecuciones',
                    data: data.map(item => item.count || 0),
                    backgroundColor: this.colors.primary,
                    borderRadius: 4
                }]
            },
            options: this.getBaseBarChartOptions()
        });
    }

    createHourlyActivityChart() {
        const ctx = document.getElementById('hourlyActivityChart')?.getContext('2d');
        if (!ctx) return;

        const data = this.performanceData.hourly_executions || [];

        this.charts.hourlyActivity = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: data.map(item => `${item.hour}:00`),
                datasets: [{
                    label: 'Ejecuciones',
                    data: data.map(item => item.execution_count || 0),
                    backgroundColor: data.map(item => {
                        const intensity = (item.execution_count || 0) / Math.max(...data.map(d => d.execution_count || 0));
                        return this.hexToRgba(this.colors.primary, 0.3 + intensity * 0.7);
                    }),
                    borderColor: this.colors.primary,
                    borderWidth: 1
                }]
            },
            options: this.getBaseBarChartOptions()
        });
    }

    createSystemResourcesChart() {
        const ctx = document.getElementById('systemResourcesChart')?.getContext('2d');
        if (!ctx) return;

        const data = this.performanceData.system_resource_usage || [];
        const latest = data[data.length - 1] || { cpu_usage: 0, memory_usage: 0, disk_usage: 0 };

        this.charts.systemResources = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: ['CPU', 'Memoria', 'Disco'],
                datasets: [{
                    data: [
                        latest.cpu_usage || 0,
                        latest.memory_usage || 0,
                        latest.disk_usage || 0
                    ],
                    backgroundColor: [
                        this.colors.danger,
                        this.colors.warning,
                        this.colors.info
                    ]
                }]
            },
            options: {
                ...this.getBaseDoughnutChartOptions(),
                plugins: {
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                return `${context.label}: ${context.parsed}%`;
                            }
                        }
                    }
                }
            }
        });
    }

    // Opciones base para gráficos
    getBaseLineChartOptions() {
        return {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: true,
                    position: 'top'
                }
            },
            scales: {
                x: {
                    grid: {
                        display: false
                    }
                },
                y: {
                    beginAtZero: true,
                    grid: {
                        color: 'rgba(0,0,0,0.1)'
                    }
                }
            }
        };
    }

    getBaseBarChartOptions() {
        return {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: false
                }
            },
            scales: {
                x: {
                    grid: {
                        display: false
                    }
                },
                y: {
                    beginAtZero: true,
                    grid: {
                        color: 'rgba(0,0,0,0.1)'
                    }
                }
            }
        };
    }

    getBaseDoughnutChartOptions() {
        return {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'bottom'
                }
            }
        };
    }

    // Métodos de utilidad
    formatTimestamp(timestamp) {
        if (!timestamp) return '';
        const date = new Date(timestamp);
        return date.toLocaleTimeString('es-ES', { 
            hour: '2-digit', 
            minute: '2-digit' 
        });
    }

    hexToRgba(hex, alpha) {
        const r = parseInt(hex.slice(1, 3), 16);
        const g = parseInt(hex.slice(3, 5), 16);
        const b = parseInt(hex.slice(5, 7), 16);
        return `rgba(${r}, ${g}, ${b}, ${alpha})`;
    }

    // Actualizar gráficos
    async updateCharts(timeRange = null) {
        if (timeRange) {
            this.currentTimeRange = timeRange;
        }

        try {
            const [performanceData, summaryData] = await Promise.all([
                this.fetchPerformanceData(this.currentTimeRange),
                this.fetchDashboardSummary()
            ]);

            this.performanceData = performanceData;
            this.summaryData = summaryData;

            // Actualizar cada gráfico
            Object.values(this.charts).forEach(chart => {
                if (chart && typeof chart.destroy === 'function') {
                    chart.destroy();
                }
            });

            // Recrear gráficos con nuevos datos
            this.createExecutionTrendChart();
            this.createSuccessRateChart();
            this.createResponseTimeChart();
            this.createQueueLengthChart();
            this.createErrorDistributionChart();
            this.createWorkflowDistributionChart();
            this.createHourlyActivityChart();
            this.createSystemResourcesChart();

            console.log('Charts updated successfully');
        } catch (error) {
            console.error('Error updating charts:', error);
        }
    }

    // Destruir todos los gráficos
    destroyCharts() {
        Object.values(this.charts).forEach(chart => {
            if (chart && typeof chart.destroy === 'function') {
                chart.destroy();
            }
        });
        this.charts = {};
    }
}

// Integración con el dashboardManager existente
if (typeof dashboardManager !== 'undefined') {
    // Extender el dashboardManager existente con funcionalidades de gráficos
    dashboardManager.chartsManager = new ChartsManager();
    
    // Sobrescribir el método de inicialización para incluir gráficos
    const originalInit = dashboardManager.init.bind(dashboardManager);
    dashboardManager.init = async function() {
        await originalInit();
        
        // Inicializar gráficos después de cargar el dashboard básico
        const chartsContainer = document.getElementById('charts-container') || 
                               document.getElementById('dashboard-charts') ||
                               document.querySelector('.dashboard-content');
        
        if (chartsContainer) {
            await this.chartsManager.initializeCharts(chartsContainer.id || 'dashboard-charts');
        }
    };

    // Extender el método de actualización para incluir gráficos
    const originalRefresh = dashboardManager.refreshData?.bind(dashboardManager);
    if (originalRefresh) {
        dashboardManager.refreshData = async function() {
            await originalRefresh();
            if (this.chartsManager) {
                await this.chartsManager.updateCharts();
            }
        };
    }
} else {
    // Si no existe dashboardManager, crear uno básico
    window.chartsManager = new ChartsManager();
    
    // Auto-inicializar cuando se carga la página
    document.addEventListener('DOMContentLoaded', () => {
        const chartsContainer = document.getElementById('dashboard-charts') || 
                               document.getElementById('charts-container') ||
                               document.querySelector('.main-content');
        
        if (chartsContainer && window.chartsManager) {
            window.chartsManager.initializeCharts(chartsContainer.id || 'dashboard-charts');
        }
    });
}

// Exportar para uso modular
if (typeof module !== 'undefined' && module.exports) {
    module.exports = ChartsManager;
}