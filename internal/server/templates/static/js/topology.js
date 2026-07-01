var topologyNetwork = null;

function topologyIsDark() {
    return document.documentElement.classList.contains('dark');
}

function topologyLabelColor() {
    return topologyIsDark() ? '#e2e8f0' : '#1e293b';
}

function isTopologyPage() {
    return !!document.getElementById('topology-container');
}

document.addEventListener('DOMContentLoaded', function() {
    if (!isTopologyPage()) return;
    loadTopology();
});

function loadTopology() {
    const container = document.getElementById('topology-container');
    if (!container) return;
    if (typeof vis === 'undefined') {
        container.innerHTML = '<div class="flex items-center justify-center h-full text-red-400"><i class="fa-solid fa-exclamation-circle mr-2"></i>vis-network 未加载</div>';
        return;
    }
    container.innerHTML = '<div class="flex items-center justify-center h-full text-slate-400 dark:text-slate-500"><i class="fa-solid fa-spinner fa-spin mr-2"></i>加载拓扑图...</div>';

    fetch('/api/topology/data')
        .then(r => r.json())
        .then(data => {
            container.innerHTML = '';
            if (!data.nodes || data.nodes.length === 0) {
                container.innerHTML = '<div class="flex items-center justify-center h-full text-slate-400 dark:text-slate-500"><i class="fa-solid fa-info-circle mr-2"></i>暂无 Implant 或监听器</div>';
                return;
            }

            const dark = topologyIsDark();
            const nodes = new vis.DataSet(data.nodes.map(n => ({
                id: n.id,
                label: n.label,
                title: n.title,
                group: n.group,
                font: { size: 12, color: topologyLabelColor() },
                borderWidth: 2,
                size: n.group === 'listener' ? 30 : 25,
            })));

            const edges = new vis.DataSet(data.edges.map(e => ({
                from: e.from,
                to: e.to,
                arrows: { to: { enabled: true, scaleFactor: 0.8 } },
                dashes: e.dashes || false,
                color: e.color || { color: dark ? '#64748b' : '#94a3b8', highlight: '#6366f1' },
                smooth: { type: 'curvedCW', roundness: 0.15 },
                width: e.width || 2,
                title: e.title || '',
            })));

            const options = {
                physics: {
                    solver: 'forceAtlas2Based',
                    forceAtlas2Based: { gravitationalConstant: -40, centralGravity: 0.005, springLength: 200, springConstant: 0.02 },
                    stabilization: { iterations: 100 },
                },
                groups: {
                    listener: { color: { background: '#6366f1', border: '#4f46e5' }, shape: 'diamond', font: { color: dark ? '#c7d2fe' : '#3730a3', size: 13, face: 'monospace' } },
                    'agent-online': { color: { background: '#10b981', border: '#059669' }, shape: 'dot', font: { color: dark ? '#6ee7b7' : '#065f46', size: 12 } },
                    'agent-offline': { color: { background: '#94a3b8', border: '#64748b' }, shape: 'dot', font: { color: dark ? '#cbd5e1' : '#475569', size: 12 } },
                },
                interaction: {
                    hover: true,
                    tooltipDelay: 200,
                    zoomView: true,
                    dragView: true,
                },
                edges: {
                    width: 2,
                    color: { inherit: false },
                },
            };

            topologyNetwork = new vis.Network(container, { nodes, edges }, options);
        })
        .catch(err => {
            container.innerHTML = '<div class="flex items-center justify-center h-full text-red-400"><i class="fa-solid fa-exclamation-circle mr-2"></i>加载失败: ' + err.message + '</div>';
        });
}