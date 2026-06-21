let refreshTimer = null;

function loadTraffic() {
    fetch('/api/traffic')
    .then(r => r.json()).then(data => {
        if (data.success) {
            renderTraffic(data.data);
        }
    }).catch(() => showToast('流量数据加载失败', 'error'));
}

function renderTraffic(logs) {
    const tbody = document.getElementById('traffic-body');
    document.getElementById('traffic-count').textContent = '最近 ' + logs.length + ' 条';

    if (logs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="py-16 text-center text-slate-400">暂无流量记录</td></tr>';
        return;
    }

    tbody.innerHTML = logs.map(log => {
        const time = new Date(log.time);
        const timeStr = time.toLocaleTimeString('zh-CN');
        const statusClass = log.status === 200 ? 'text-emerald-600 bg-emerald-100' :
                           log.status >= 400 ? 'text-red-600 bg-red-100' : 'text-amber-600 bg-amber-100';
        return '<tr class="hover:bg-slate-50 transition-colors font-mono text-xs">' +
            '<td class="py-2 px-4 text-slate-500">' + timeStr + '</td>' +
            '<td class="py-2 px-4"><span class="px-1.5 py-0.5 rounded text-xs font-medium ' + (log.method === 'POST' ? 'bg-indigo-100 text-indigo-700' : 'bg-emerald-100 text-emerald-700') + '">' + log.method + '</span></td>' +
            '<td class="py-2 px-4 text-slate-700 max-w-xs truncate">' + escapeHtml(log.path) + '</td>' +
            '<td class="py-2 px-4 text-slate-500">' + escapeHtml(log.remote_ip) + '</td>' +
            '<td class="py-2 px-4 text-slate-500">' + (log.agent_id ? log.agent_id.substring(0, 8) + '...' : '-') + '</td>' +
            '<td class="py-2 px-4 text-center"><span class="px-1.5 py-0.5 rounded text-xs ' + statusClass + '">' + log.status + '</span></td>' +
            '<td class="py-2 px-4 text-right text-slate-500">' + (log.size > 0 ? log.size + 'B' : '-') + '</td>' +
            '<td class="py-2 px-4 text-right text-slate-500">' + log.latency + '</td>' +
            '</tr>';
    }).join('');
}

function toggleAutoRefresh() {
    if (document.getElementById('auto-refresh').checked) {
        refreshTimer = setInterval(loadTraffic, 3000);
    } else {
        if (refreshTimer) clearInterval(refreshTimer);
        refreshTimer = null;
    }
}

document.addEventListener('DOMContentLoaded', function() {
    loadTraffic();
    refreshTimer = setInterval(loadTraffic, 3000);
});
