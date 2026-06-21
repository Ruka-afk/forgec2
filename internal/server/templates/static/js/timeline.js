// Timeline page - load, filter, and export event timeline

let allEvents = [];

document.addEventListener('DOMContentLoaded', function() {
    loadTimeline();
});

function loadTimeline() {
    document.getElementById('timeline-loading').classList.remove('hidden');
    document.getElementById('timeline-list').classList.add('hidden');
    document.getElementById('timeline-empty').classList.add('hidden');

    const params = new URLSearchParams();
    const type = document.getElementById('filter-type').value;
    const user = document.getElementById('filter-user').value;
    const agent = document.getElementById('filter-agent').value;

    if (type) params.set('type', type);
    if (user) params.set('user', user);
    if (agent) params.set('agent', agent);

    fetch('/api/timeline/data?' + params.toString())
        .then(r => r.json())
        .then(data => {
            allEvents = data.events || [];
            renderTimeline(allEvents);
            updateStats(allEvents);
        })
        .catch(err => {
            console.error('Failed to load timeline:', err);
            showToast('加载时间线失败', 'error');
        });
}

function renderTimeline(events) {
    const container = document.getElementById('timeline-list');
    const loading = document.getElementById('timeline-loading');
    const empty = document.getElementById('timeline-empty');

    loading.classList.add('hidden');

    if (events.length === 0) {
        container.classList.add('hidden');
        empty.classList.remove('hidden');
        return;
    }

    empty.classList.add('hidden');
    container.classList.remove('hidden');

    let html = '<div class="space-y-4">';

    events.forEach((event, index) => {
        const time = new Date(event.timestamp);
        const timeStr = time.toLocaleString('zh-CN', {
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit'
        });

        const isAudit = event.type === 'audit';
        const icon = isAudit ? 'fa-shield-halved' : 'fa-terminal';
        const iconBg = isAudit ? 'bg-blue-100 text-blue-600' : 'bg-emerald-100 text-emerald-600';
        const badgeColor = isAudit ? 'bg-blue-50 text-blue-700' : 'bg-emerald-50 text-emerald-700';
        const badgeText = isAudit ? '审计' : '任务';

        const successBadge = event.success
            ? '<span class="text-xs px-2 py-0.5 bg-emerald-50 text-emerald-700 rounded-md font-medium">成功</span>'
            : '<span class="text-xs px-2 py-0.5 bg-red-50 text-red-700 rounded-md font-medium">失败</span>';

        html += `
        <div class="flex gap-4 group">
            <!-- 时间线节点 -->
            <div class="flex flex-col items-center">
                <div class="w-10 h-10 ${iconBg} rounded-full flex items-center justify-center flex-shrink-0">
                    <i class="fa-solid ${icon}"></i>
                </div>
                ${index < events.length - 1 ? '<div class="w-0.5 h-full bg-slate-200 mt-2"></div>' : ''}
            </div>

            <!-- 事件卡片 -->
            <div class="flex-1 bg-slate-50 border border-slate-200 rounded-xl p-4 hover:bg-slate-100 transition-colors">
                <div class="flex items-start justify-between mb-2">
                    <div class="flex items-center gap-2">
                        <span class="text-xs font-mono text-slate-600">${timeStr}</span>
                        <span class="text-xs px-2 py-0.5 ${badgeColor} rounded-md font-medium">${badgeText}</span>
                        ${event.agent_id ? `<a href="/agents/${event.agent_id}" class="text-xs px-2 py-0.5 bg-indigo-50 text-indigo-700 rounded-md font-medium hover:bg-indigo-100 transition-colors">${event.agent_name || event.agent_id}</a>` : ''}
                    </div>
                    <div class="flex items-center gap-2">
                        ${successBadge}
                        ${event.ip ? `<span class="text-xs text-slate-500 font-mono">${event.ip}</span>` : ''}
                    </div>
                </div>

                <div class="flex items-start justify-between gap-4">
                    <div class="flex-1">
                        <div class="text-sm font-medium text-slate-900 mb-1">${event.action}</div>
                        ${event.details ? `<div class="text-xs text-slate-600 font-mono bg-white border border-slate-200 rounded-lg p-2 mt-2 max-h-20 overflow-auto">${escapeHtml(event.details)}</div>` : ''}
                    </div>
                    <div class="text-right">
                        <div class="text-xs text-slate-500">
                            <i class="fa-solid fa-user mr-1"></i>${event.user || 'system'}
                        </div>
                    </div>
                </div>
            </div>
        </div>
        `;
    });

    html += '</div>';
    container.innerHTML = html;
}

function updateStats(events) {
    document.getElementById('total-events').textContent = events.length;

    const auditCount = events.filter(e => e.type === 'audit').length;
    const taskCount = events.filter(e => e.type === 'task').length;

    document.getElementById('audit-count').textContent = auditCount;
    document.getElementById('task-count').textContent = taskCount;
}

function applyFilters() {
    loadTimeline();
}

function clearFilters() {
    document.getElementById('filter-type').value = '';
    document.getElementById('filter-user').value = '';
    document.getElementById('filter-agent').value = '';
    loadTimeline();
}

function exportTimeline() {
    const params = new URLSearchParams();
    const type = document.getElementById('filter-type').value;
    const user = document.getElementById('filter-user').value;
    const agent = document.getElementById('filter-agent').value;

    if (type) params.set('type', type);
    if (user) params.set('user', user);
    if (agent) params.set('agent', agent);

    window.open('/api/timeline/export?' + params.toString(), '_blank');
    showToast('正在导出时间线...', 'success');
}
