// Data Visualization Module
const DataViz = (function() {
    
    // Render bar chart
    function renderBarChart(container, labels, values, title, color = '#4F46E5') {
        if (!container || labels.length === 0) return;
        
        const maxValue = Math.max(...values);
        
        container.innerHTML = `
            <div class="mb-4">
                <h3 class="text-lg font-semibold text-slate-300 mb-3">${title}</h3>
                <div class="space-y-2">
                    ${labels.map((label, i) => `
                        <div class="flex items-center gap-3">
                            <span class="text-sm text-slate-400 w-24 text-right">${label}</span>
                            <div class="flex-1 bg-slate-700 rounded-full h-6 overflow-hidden">
                                <div class="h-full rounded-full transition-all duration-500" 
                                     style="width: ${(values[i] / maxValue) * 100}%; background: ${color}">
                                </div>
                            </div>
                            <span class="text-sm text-slate-300 w-16">${values[i]}</span>
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
    }
    
    // Render pie chart (simple version)
    function renderPieChart(container, labels, values, title) {
        if (!container || labels.length === 0) return;
        
        const total = values.reduce((a, b) => a + b, 0);
        const colors = ['#4F46E5', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6', '#EC4899'];
        
        let currentAngle = 0;
        const segments = values.map((val, i) => {
            const angle = (val / total) * 360;
            const segment = {
                label: labels[i],
                value: val,
                percentage: ((val / total) * 100).toFixed(1),
                color: colors[i % colors.length],
                startAngle: currentAngle,
                endAngle: currentAngle + angle
            };
            currentAngle += angle;
            return segment;
        });
        
        container.innerHTML = `
            <div class="mb-4">
                <h3 class="text-lg font-semibold text-slate-300 mb-3">${title}</h3>
                <div class="flex gap-4">
                    <div class="w-48 h-48 rounded-full relative overflow-hidden" style="background: conic-gradient(${
                        segments.map(s => `${s.color} ${s.startAngle}deg ${s.endAngle}deg`).join(', ')
                    })">
                        <div class="absolute inset-8 bg-slate-800 rounded-full flex items-center justify-center">
                            <div class="text-center">
                                <div class="text-2xl font-bold text-white">${total}</div>
                                <div class="text-xs text-slate-400">总计</div>
                            </div>
                        </div>
                    </div>
                    <div class="flex-1 space-y-2">
                        ${segments.map(s => `
                            <div class="flex items-center gap-2">
                                <div class="w-3 h-3 rounded" style="background: ${s.color}"></div>
                                <span class="text-sm text-slate-300 flex-1">${s.label}</span>
                                <span class="text-sm text-slate-400">${s.value} (${s.percentage}%)</span>
                            </div>
                        `).join('')}
                    </div>
                </div>
            </div>
        `;
    }
    
    // Render line chart (simple)
    function renderLineChart(container, labels, values, title, color = '#4F46E5') {
        if (!container || labels.length === 0) return;
        
        const maxValue = Math.max(...values);
        const minValue = Math.min(...values);
        const range = maxValue - minValue || 1;
        
        const width = 600;
        const height = 200;
        const padding = 40;
        
        const points = values.map((val, i) => {
            const x = padding + (i / (values.length - 1)) * (width - padding * 2);
            const y = height - padding - ((val - minValue) / range) * (height - padding * 2);
            return { x, y, val, label: labels[i] };
        });
        
        const pathD = points.map((p, i) => {
            return i === 0 ? `M ${p.x},${p.y}` : `L ${p.x},${p.y}`;
        }).join(' ');
        
        container.innerHTML = `
            <div class="mb-4">
                <h3 class="text-lg font-semibold text-slate-300 mb-3">${title}</h3>
                <svg width="${width}" height="${height}" class="bg-slate-900 rounded">
                    <path d="${pathD}" stroke="${color}" stroke-width="2" fill="none"/>
                    ${points.map(p => `
                        <circle cx="${p.x}" cy="${p.y}" r="4" fill="${color}"/>
                        <text x="${p.x}" y="${p.y - 10}" text-anchor="middle" fill="#94A3B8" font-size="10">${p.val}</text>
                    `).join('')}
                </svg>
            </div>
        `;
    }
    
    // Render heatmap (calendar style)
    function renderHeatmap(container, data, title) {
        if (!container) return;
        
        // data: {date: '2026-01-01', count: 5}
        const maxCount = Math.max(...data.map(d => d.count));
        
        const getColor = (count) => {
            if (count === 0) return 'bg-slate-800';
            const intensity = Math.min(count / maxCount, 1);
            if (intensity < 0.25) return 'bg-indigo-900';
            if (intensity < 0.5) return 'bg-indigo-700';
            if (intensity < 0.75) return 'bg-indigo-500';
            return 'bg-indigo-400';
        };
        
        container.innerHTML = `
            <div class="mb-4">
                <h3 class="text-lg font-semibold text-slate-300 mb-3">${title}</h3>
                <div class="flex flex-wrap gap-1">
                    ${data.map(d => `
                        <div class="w-4 h-4 rounded ${getColor(d.count)}" 
                             title="${d.date}: ${d.count} 次"></div>
                    `).join('')}
                </div>
                <div class="flex items-center gap-2 mt-2 text-xs text-slate-500">
                    <span>少</span>
                    <div class="w-4 h-4 bg-slate-800 rounded"></div>
                    <div class="w-4 h-4 bg-indigo-900 rounded"></div>
                    <div class="w-4 h-4 bg-indigo-700 rounded"></div>
                    <div class="w-4 h-4 bg-indigo-500 rounded"></div>
                    <div class="w-4 h-4 bg-indigo-400 rounded"></div>
                    <span>多</span>
                </div>
            </div>
        `;
    }
    
    // Render Sankey diagram (simplified)
    function renderSankey(container, flows, title) {
        // flows: [{from: 'Agent1', to: 'Task1', value: 10}]
        if (!container) return;
        
        container.innerHTML = `
            <div class="mb-4">
                <h3 class="text-lg font-semibold text-slate-300 mb-3">${title}</h3>
                <div class="bg-slate-900 p-4 rounded">
                    <div class="text-sm text-slate-400">
                        Sankey 图需要更复杂的 SVG 渲染库。
                        当前显示简化版本。
                    </div>
                    <div class="mt-4 space-y-2">
                        ${flows.map(f => `
                            <div class="flex items-center gap-2 text-sm">
                                <span class="text-slate-300">${f.from}</span>
                                <i class="fa-solid fa-arrow-right text-slate-500"></i>
                                <span class="text-slate-300">${f.to}</span>
                                <span class="text-indigo-400">${f.value}</span>
                            </div>
                        `).join('')}
                    </div>
                </div>
            </div>
        `;
    }
    
    // Render metric cards
    function renderMetricCards(container, metrics) {
        // metrics: [{label: '在线 Agent', value: '15', icon: 'fa-server', color: 'indigo'}]
        if (!container) return;
        
        const colorMap = {
            'indigo': 'bg-indigo-600',
            'green': 'bg-green-600',
            'red': 'bg-red-600',
            'yellow': 'bg-yellow-600',
            'blue': 'bg-blue-600',
            'purple': 'bg-purple-600'
        };
        
        container.innerHTML = `
            <div class="grid grid-cols-4 gap-4 mb-6">
                ${metrics.map(m => `
                    <div class="bg-slate-800 rounded-lg p-4 border border-slate-700">
                        <div class="flex items-center gap-3 mb-2">
                            <div class="${colorMap[m.color] || colorMap['indigo']} w-10 h-10 rounded-lg flex items-center justify-center">
                                <i class="fa-solid ${m.icon} text-white"></i>
                            </div>
                            <div>
                                <div class="text-2xl font-bold text-white">${m.value}</div>
                                <div class="text-xs text-slate-400">${m.label}</div>
                            </div>
                        </div>
                    </div>
                `).join('')}
            </div>
        `;
    }
    
    // Render timeline
    function renderTimeline(container, events, title) {
        // events: [{time: '10:00', title: '事件', description: '描述', type: 'success'}]
        if (!container) return;
        
        const typeColors = {
            'success': 'bg-green-500',
            'error': 'bg-red-500',
            'warning': 'bg-yellow-500',
            'info': 'bg-blue-500'
        };
        
        container.innerHTML = `
            <div class="mb-4">
                <h3 class="text-lg font-semibold text-slate-300 mb-3">${title}</h3>
                <div class="relative pl-8 border-l-2 border-slate-700 space-y-4">
                    ${events.map(e => `
                        <div class="relative">
                            <div class="absolute -left-10 w-6 h-6 rounded-full ${typeColors[e.type] || typeColors['info']} flex items-center justify-center">
                                <i class="fa-solid fa-circle text-white text-xs"></i>
                            </div>
                            <div class="bg-slate-800 rounded-lg p-3">
                                <div class="flex items-center justify-between mb-1">
                                    <span class="font-semibold text-slate-300">${e.title}</span>
                                    <span class="text-xs text-slate-500">${e.time}</span>
                                </div>
                                <div class="text-sm text-slate-400">${e.description}</div>
                            </div>
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
    }
    
    // Export chart as image
    function exportAsImage(container, filename = 'chart.png') {
        if (!container) return;
        
        // For SVG elements
        const svg = container.querySelector('svg');
        if (svg) {
            const svgData = new XMLSerializer().serializeToString(svg);
            const canvas = document.createElement('canvas');
            const ctx = canvas.getContext('2d');
            const img = new Image();
            
            canvas.width = svg.clientWidth;
            canvas.height = svg.clientHeight;
            
            img.onload = () => {
                ctx.drawImage(img, 0, 0);
                const link = document.createElement('a');
                link.download = filename;
                link.href = canvas.toDataURL('image/png');
                link.click();
            };
            
            img.src = 'data:image/svg+xml;base64,' + btoa(svgData);
        }
    }
    
    return {
        renderBarChart,
        renderPieChart,
        renderLineChart,
        renderHeatmap,
        renderSankey,
        renderMetricCards,
        renderTimeline,
        exportAsImage
    };
})();

// Export to global scope
window.DataViz = DataViz;
