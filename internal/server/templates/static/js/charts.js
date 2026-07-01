const ForgeCharts = {
    charts: {},
    chartJSReady: false,
    colorScheme: {
        light: {
            text: '#1e293b',
            textSecondary: '#64748b',
            grid: '#e2e8f0',
            background: '#ffffff',
            cardBg: '#f8fafc',
            border: '#e2e8f0'
        },
        dark: {
            text: '#f1f5f9',
            textSecondary: '#94a3b8',
            grid: '#334155',
            background: '#0f172a',
            cardBg: '#1e293b',
            border: '#334155'
        }
    },
    colors: [
        '#6366f1', '#8b5cf6', '#ec4899', '#ef4444',
        '#f97316', '#eab308', '#22c55e', '#14b8a6',
        '#06b6d4', '#3b82f6'
    ],

    isDarkMode() {
        return document.documentElement.classList.contains('dark');
    },

    getColors() {
        return this.isDarkMode() ? this.colorScheme.dark : this.colorScheme.light;
    },

    getChartColors() {
        return this.colors;
    },

    createGradient(ctx, color, height) {
        const gradient = ctx.createLinearGradient(0, 0, 0, height);
        gradient.addColorStop(0, color + '40');
        gradient.addColorStop(1, color + '05');
        return gradient;
    },

    getBaseOptions() {
        const colors = this.getColors();
        const isDark = this.isDarkMode();
        return {
            responsive: true,
            maintainAspectRatio: false,
            interaction: {
                mode: 'index',
                intersect: false,
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'bottom',
                    labels: {
                        color: colors.text,
                        padding: 15,
                        usePointStyle: true,
                        pointStyle: 'circle',
                        font: { size: 11 }
                    }
                },
                tooltip: {
                    backgroundColor: isDark ? 'rgba(30, 41, 59, 0.95)' : 'rgba(255, 255, 255, 0.95)',
                    titleColor: colors.text,
                    bodyColor: colors.textSecondary,
                    borderColor: colors.border,
                    borderWidth: 1,
                    padding: 12,
                    cornerRadius: 8,
                    titleFont: { size: 13, weight: '600' },
                    bodyFont: { size: 12 }
                }
            },
            scales: {
                x: {
                    grid: {
                        color: colors.grid,
                        drawBorder: false
                    },
                    ticks: {
                        color: colors.textSecondary,
                        font: { size: 11 }
                    }
                },
                y: {
                    grid: {
                        color: colors.grid,
                        drawBorder: false
                    },
                    ticks: {
                        color: colors.textSecondary,
                        font: { size: 11 }
                    }
                }
            }
        };
    },

    createLineChart(canvasId, data, options = {}) {
        if (typeof Chart === 'undefined') {
            console.warn('Chart.js not loaded, cannot create line chart');
            return null;
        }
        const canvas = document.getElementById(canvasId);
        if (!canvas) return null;
        const ctx = canvas.getContext('2d');
        if (this.charts[canvasId]) {
            this.charts[canvasId].destroy();
        }
        const baseOptions = this.getBaseOptions();
        const mergedOptions = this.deepMerge(baseOptions, options);
        const datasets = data.datasets.map((ds, i) => {
            const color = ds.borderColor || this.colors[i % this.colors.length];
            return {
                ...ds,
                borderColor: color,
                backgroundColor: ds.fill ? this.createGradient(ctx, color, canvas.height) : color + '20',
                tension: ds.tension || 0.4,
                borderWidth: ds.borderWidth || 2,
                pointRadius: ds.pointRadius || 0,
                pointHoverRadius: ds.pointHoverRadius || 5,
                pointHoverBackgroundColor: color,
                pointHoverBorderColor: '#fff',
                pointHoverBorderWidth: 2
            };
        });
        this.charts[canvasId] = new Chart(ctx, {
            type: 'line',
            data: { labels: data.labels, datasets },
            options: mergedOptions
        });
        return this.charts[canvasId];
    },

    createBarChart(canvasId, data, options = {}) {
        if (typeof Chart === 'undefined') {
            console.warn('Chart.js not loaded, cannot create bar chart');
            return null;
        }
        const canvas = document.getElementById(canvasId);
        if (!canvas) return null;
        const ctx = canvas.getContext('2d');
        if (this.charts[canvasId]) {
            this.charts[canvasId].destroy();
        }
        const baseOptions = this.getBaseOptions();
        const mergedOptions = this.deepMerge(baseOptions, options);
        const datasets = data.datasets.map((ds, i) => {
            const color = ds.backgroundColor || this.colors[i % this.colors.length];
            return {
                ...ds,
                backgroundColor: ds.backgroundColor || color + 'cc',
                borderColor: color,
                borderWidth: ds.borderWidth || 0,
                borderRadius: ds.borderRadius || 4,
                barPercentage: ds.barPercentage || 0.7
            };
        });
        this.charts[canvasId] = new Chart(ctx, {
            type: 'bar',
            data: { labels: data.labels, datasets },
            options: mergedOptions
        });
        return this.charts[canvasId];
    },

    createPieChart(canvasId, data, options = {}) {
        if (typeof Chart === 'undefined') {
            console.warn('Chart.js not loaded, cannot create pie chart');
            return null;
        }
        const canvas = document.getElementById(canvasId);
        if (!canvas) return null;
        const ctx = canvas.getContext('2d');
        if (this.charts[canvasId]) {
            this.charts[canvasId].destroy();
        }
        const colors = this.getColors();
        const baseOptions = {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: true,
                    position: 'right',
                    labels: {
                        color: colors.text,
                        padding: 12,
                        usePointStyle: true,
                        pointStyle: 'circle',
                        font: { size: 11 }
                    }
                },
                tooltip: {
                    backgroundColor: this.isDarkMode() ? 'rgba(30, 41, 59, 0.95)' : 'rgba(255, 255, 255, 0.95)',
                    titleColor: colors.text,
                    bodyColor: colors.textSecondary,
                    borderColor: colors.border,
                    borderWidth: 1,
                    padding: 12,
                    cornerRadius: 8
                }
            }
        };
        const mergedOptions = this.deepMerge(baseOptions, options);
        const backgroundColors = data.datasets[0].backgroundColor || 
            data.labels.map((_, i) => this.colors[i % this.colors.length]);
        const datasets = data.datasets.map(ds => ({
            ...ds,
            backgroundColor: backgroundColors,
            borderColor: colors.background,
            borderWidth: 2,
            hoverOffset: 8
        }));
        this.charts[canvasId] = new Chart(ctx, {
            type: 'pie',
            data: { labels: data.labels, datasets },
            options: mergedOptions
        });
        return this.charts[canvasId];
    },

    createDoughnutChart(canvasId, data, options = {}) {
        if (typeof Chart === 'undefined') {
            console.warn('Chart.js not loaded, cannot create doughnut chart');
            return null;
        }
        const canvas = document.getElementById(canvasId);
        if (!canvas) return null;
        const ctx = canvas.getContext('2d');
        if (this.charts[canvasId]) {
            this.charts[canvasId].destroy();
        }
        const colors = this.getColors();
        const baseOptions = {
            responsive: true,
            maintainAspectRatio: false,
            cutout: '65%',
            plugins: {
                legend: {
                    display: true,
                    position: 'right',
                    labels: {
                        color: colors.text,
                        padding: 12,
                        usePointStyle: true,
                        pointStyle: 'circle',
                        font: { size: 11 }
                    }
                },
                tooltip: {
                    backgroundColor: this.isDarkMode() ? 'rgba(30, 41, 59, 0.95)' : 'rgba(255, 255, 255, 0.95)',
                    titleColor: colors.text,
                    bodyColor: colors.textSecondary,
                    borderColor: colors.border,
                    borderWidth: 1,
                    padding: 12,
                    cornerRadius: 8
                }
            }
        };
        const mergedOptions = this.deepMerge(baseOptions, options);
        const backgroundColors = data.datasets[0].backgroundColor || 
            data.labels.map((_, i) => this.colors[i % this.colors.length]);
        const datasets = data.datasets.map(ds => ({
            ...ds,
            backgroundColor: backgroundColors,
            borderColor: colors.background,
            borderWidth: 2,
            hoverOffset: 8
        }));
        this.charts[canvasId] = new Chart(ctx, {
            type: 'doughnut',
            data: { labels: data.labels, datasets },
            options: mergedOptions
        });
        return this.charts[canvasId];
    },

    createGaugeChart(canvasId, value, maxValue, color, label) {
        const canvas = document.getElementById(canvasId);
        if (!canvas) return null;
        const ctx = canvas.getContext('2d');
        if (this.charts[canvasId]) {
            this.charts[canvasId].destroy();
        }
        const colors = this.getColors();
        const percent = Math.min(100, (value / maxValue) * 100);
        const data = {
            datasets: [{
                data: [percent, 100 - percent],
                backgroundColor: [color, colors.grid],
                borderWidth: 0,
                circumference: 180,
                rotation: 270
            }]
        };
        this.charts[canvasId] = new Chart(ctx, {
            type: 'doughnut',
            data: data,
            options: {
                responsive: true,
                maintainAspectRatio: false,
                cutout: '75%',
                plugins: {
                    legend: { display: false },
                    tooltip: { enabled: false }
                }
            },
            plugins: [{
                id: 'gaugeText',
                beforeDraw: function(chart) {
                    const { ctx, chartArea: { left, right, top, bottom } } = chart;
                    const centerX = (left + right) / 2;
                    const centerY = (top + bottom) / 2 + 10;
                    ctx.save();
                    ctx.textAlign = 'center';
                    ctx.textBaseline = 'middle';
                    ctx.fillStyle = colors.text;
                    ctx.font = 'bold 24px system-ui';
                    ctx.fillText(value.toFixed(1) + '%', centerX, centerY - 5);
                    ctx.fillStyle = colors.textSecondary;
                    ctx.font = '11px system-ui';
                    ctx.fillText(label, centerX, centerY + 15);
                    ctx.restore();
                }
            }]
        });
        return this.charts[canvasId];
    },

    createHeatmap(containerId, data, options = {}) {
        const container = document.getElementById(containerId);
        if (!container) return null;
        const days = options.days || 7;
        const hours = options.hours || 24;
        const maxValue = Math.max(...data.map(d => d.value), 1);
        let html = '<div class="heatmap-container">';
        html += '<div class="heatmap-labels">';
        const dayLabels = [__t('Mon'), __t('Tue'), __t('Wed'), __t('Thu'), __t('Fri'), __t('Sat'), __t('Sun')];
        for (let d = 0; d < days; d++) {
            html += `<div class="heatmap-day-label">${dayLabels[d % 7]}</div>`;
        }
        html += '</div>';
        html += '<div class="heatmap-grid">';
        html += '<div class="heatmap-hour-labels">';
        for (let h = 0; h < hours; h += 4) {
            html += `<div class="heatmap-hour-label">${h}:00</div>`;
        }
        html += '</div>';
        html += '<div class="heatmap-cells">';
        for (let d = 0; d < days; d++) {
            for (let h = 0; h < hours; h++) {
                const item = data.find(x => x.day === d && x.hour === h);
                const value = item ? item.value : 0;
                const intensity = value / maxValue;
                const color = this.getHeatmapColor(intensity);
                html += `<div class="heatmap-cell" style="background-color: ${color}" title="${dayLabels[d % 7]} ${h}:00 - ${value} ${__t('activity')}" data-day="${d}" data-hour="${h}" data-value="${value}"></div>`;
            }
        }
        html += '</div></div></div>';
        html += '<div class="heatmap-legend mt-2 flex items-center justify-end gap-2 text-xs text-slate-500">';
        html += '<span>' + __t('Low') + '</span>';
        html += '<div class="flex gap-0.5">';
        for (let i = 0; i <= 4; i++) {
            html += `<div class="w-3 h-3 rounded-sm" style="background-color: ${this.getHeatmapColor(i / 4)}"></div>`;
        }
        html += '</div>';
        html += '<span>' + __t('High') + '</span>';
        html += '</div>';
        container.innerHTML = html;
        return container;
    },

    getHeatmapColor(intensity) {
        if (intensity <= 0) return this.isDarkMode() ? '#1e293b' : '#f1f5f9';
        const r = Math.round(99 + (99 * (1 - intensity)));
        const g = Math.round(102 + (153 * (1 - intensity)));
        const b = Math.round(241 + (14 * (1 - intensity)));
        return `rgb(${r}, ${g}, ${b})`;
    },

    updateChart(canvasId, data) {
        if (this.charts[canvasId]) {
            this.charts[canvasId].data = data;
            this.charts[canvasId].update('none');
        }
    },

    destroyChart(canvasId) {
        if (this.charts[canvasId]) {
            this.charts[canvasId].destroy();
            delete this.charts[canvasId];
        }
    },

    destroyAll() {
        Object.keys(this.charts).forEach(id => this.destroyChart(id));
    },

    exportPNG(canvasId, filename = 'chart.png') {
        const chart = this.charts[canvasId];
        if (!chart) return;
        const link = document.createElement('a');
        link.download = filename;
        link.href = chart.toBase64Image('image/png', 1);
        link.click();
    },

    deepMerge(target, source) {
        const output = { ...target };
        if (this.isObject(target) && this.isObject(source)) {
            Object.keys(source).forEach(key => {
                if (this.isObject(source[key])) {
                    if (!(key in target)) {
                        Object.assign(output, { [key]: source[key] });
                    } else {
                        output[key] = this.deepMerge(target[key], source[key]);
                    }
                } else {
                    Object.assign(output, { [key]: source[key] });
                }
            });
        }
        return output;
    },

    isObject(item) {
        return (item && typeof item === 'object' && !Array.isArray(item));
    }
};
