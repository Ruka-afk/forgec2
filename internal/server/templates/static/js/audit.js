// Audit log page - filter, paginate, export

let currentPage = 1;
let totalPages = 1;
let totalRecords = 0;
const pageSize = 50;

function loadLogs() {
    const search = document.getElementById('search-input').value;
    const action = document.getElementById('action-filter').value;
    const user = document.getElementById('user-filter').value;

    fetch(`/audit/logs?page=${currentPage}&pageSize=${pageSize}&search=${encodeURIComponent(search)}&action=${action}&user=${encodeURIComponent(user)}`)
    .then(r => r.json()).then(data => {
        if (data.success) {
            renderLogs(data.data);
            totalRecords = data.total;
            totalPages = Math.ceil(totalRecords / pageSize);
            updatePagination();
        } else {
            showError(data.error || '加载失败');
        }
    }).catch(err => showError('网络错误: ' + err));
}

function renderLogs(logs) {
    const tbody = document.getElementById('log-table-body');

    if (logs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="py-16 text-center text-slate-400">暂无审计记录</td></tr>';
        return;
    }

    tbody.innerHTML = logs.map(log => `
        <tr class="hover:bg-slate-50 transition-colors">
            <td class="py-3 px-4 text-sm text-slate-600">${formatTime(log.created_at)}</td>
            <td class="py-3 px-4 text-sm">
                <span class="px-2 py-1 bg-indigo-100 text-indigo-700 rounded-lg text-xs">${escapeHtml(log.user)}</span>
            </td>
            <td class="py-3 px-4 text-sm text-slate-600 font-mono">${escapeHtml(log.ip) || '-'}</td>
            <td class="py-3 px-4 text-sm">
                <span class="px-2 py-1 ${getActionClass(log.action)} rounded-lg text-xs">${getActionLabel(log.action)}</span>
            </td>
            <td class="py-3 px-4 text-sm text-slate-600">${escapeHtml(log.resource) || '-'}</td>
            <td class="py-3 px-4 text-sm text-slate-600 font-mono">${log.agent_id ? log.agent_id.substring(0, 8) + '...' : '-'}</td>
            <td class="py-3 px-4 text-sm">
                <span class="flex items-center">
                    ${log.success ? '<i class="fa-solid fa-check-circle text-emerald-500 mr-1"></i><span class="text-emerald-600">成功</span>' : '<i class="fa-solid fa-x-circle text-red-500 mr-1"></i><span class="text-red-600">失败</span>'}
                </span>
            </td>
            <td class="py-3 px-4 text-sm text-slate-500 max-w-xs truncate" title="${escapeHtml(log.details || log.error || '-')}">
                ${escapeHtml(log.details || log.error || '-')}
            </td>
        </tr>
    `).join('');
}

function formatTime(timeStr) {
    if (!timeStr) return '-';
    const date = new Date(timeStr);
    return date.toLocaleString('zh-CN');
}

function getActionClass(action) {
    const classes = {
        'login': 'bg-emerald-100 text-emerald-700',
        'logout': 'bg-slate-100 text-slate-700',
        'send_command': 'bg-indigo-100 text-indigo-700',
        'command_result': 'bg-teal-100 text-teal-700',
        'batch_command': 'bg-violet-100 text-violet-700',
        'agent_action': 'bg-blue-100 text-blue-700',
        'kill_agent': 'bg-red-100 text-red-700',
        'delete_agent': 'bg-rose-100 text-rose-700',
        'request_screenshot': 'bg-sky-100 text-sky-700',
        'request_ps': 'bg-amber-100 text-amber-700',
        'generate': 'bg-purple-100 text-purple-700',
        'screen_monitor_start': 'bg-cyan-100 text-cyan-700',
        'screen_monitor_stop': 'bg-orange-100 text-orange-700',
        'file_ls': 'bg-slate-100 text-slate-700',
        'file_delete': 'bg-red-100 text-red-700',
        'file_read': 'bg-blue-100 text-blue-700',
        'file_upload_exfil': 'bg-emerald-100 text-emerald-700',
        'file_download_exfil': 'bg-emerald-100 text-emerald-700',
    };
    return classes[action] || 'bg-gray-100 text-gray-700';
}

function getActionLabel(action) {
    const labels = {
        'login': '登录',
        'logout': '退出',
        'send_command': '发送命令',
        'command_result': '命令结果',
        'batch_command': '批量命令',
        'agent_action': 'Implant 操作',
        'kill_agent': '终止 Implant',
        'delete_agent': '删除 Implant',
        'request_screenshot': '请求截图',
        'request_ps': '请求进程列表',
        'generate': '生成 Implant',
        'screen_monitor_start': '启动监控',
        'screen_monitor_stop': '停止监控',
        'file_ls': '文件列表',
        'file_delete': '文件删除',
        'file_read': '文件读取',
        'file_upload_exfil': '文件上传(内传)',
        'file_download_exfil': '文件下载(外传)',
    };
    return labels[action] || action;
}

function updatePagination() {
    document.getElementById('total-count').textContent = `共 ${totalRecords} 条记录`;
    document.getElementById('current-page').textContent = currentPage;
    document.getElementById('prev-page').disabled = currentPage <= 1;
    document.getElementById('next-page').disabled = currentPage >= totalPages;

    const start = (currentPage - 1) * pageSize + 1;
    const end = Math.min(currentPage * pageSize, totalRecords);
    document.getElementById('page-info').textContent = `${start}-${end}`;
}

function prevPage() {
    if (currentPage > 1) {
        currentPage--;
        loadLogs();
    }
}

function nextPage() {
    if (currentPage < totalPages) {
        currentPage++;
        loadLogs();
    }
}

function applyFilters() {
    currentPage = 1;
    loadLogs();
}

function resetFilters() {
    document.getElementById('search-input').value = '';
    document.getElementById('action-filter').value = '';
    document.getElementById('user-filter').value = '';
    currentPage = 1;
    loadLogs();
}

function exportLogs() {
    const search = document.getElementById('search-input').value;
    const action = document.getElementById('action-filter').value;
    const user = document.getElementById('user-filter').value;

    fetch(`/audit/logs?page=1&pageSize=${totalRecords}&search=${encodeURIComponent(search)}&action=${action}&user=${encodeURIComponent(user)}`)
    .then(r => r.json()).then(data => {
        if (data.success) {
            const csv = convertToCSV(data.data);
            downloadCSV(csv);
        } else {
            showError(data.error || '导出失败');
        }
    }).catch(err => showError('网络错误: ' + err));
}

function convertToCSV(logs) {
    const headers = ['时间', '用户', 'IP', '操作', '资源', 'Implant', '状态', '详情'];
    const rows = logs.map(log => [
        formatTime(log.created_at),
        log.user,
        log.ip || '',
        getActionLabel(log.action),
        log.resource || '',
        log.agent_id || '',
        log.success ? '成功' : '失败',
        log.details || log.error || ''
    ]);

    return [headers.join(','), ...rows.map(row => row.map(cell => `"${cell.replace(/"/g, '""')}"`).join(','))].join('\n');
}

function downloadCSV(content) {
    const blob = new Blob([`\uFEFF${content}`], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = `audit_logs_${new Date().toISOString().split('T')[0]}.csv`;
    link.click();
}

function showError(message) {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'px-4 py-3 rounded-2xl shadow-xl flex items-center gap-x-3 text-sm bg-red-100 border border-red-300 text-red-700';
    toast.innerHTML = '<i class="fa-solid fa-exclamation-circle"></i>';
    const msgSpan = document.createElement('span');
    msgSpan.textContent = message;
    toast.appendChild(msgSpan);
    container.appendChild(toast);
    setTimeout(() => {
        toast.style.transition = 'all 0.3s ease';
        toast.style.opacity = '0';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

document.getElementById('search-input').addEventListener('keyup', function(e) {
    if (e.key === 'Enter') applyFilters();
});

loadLogs();
