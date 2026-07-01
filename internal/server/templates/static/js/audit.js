let currentPage = 1;
let totalPages = 1;
let totalRecords = 0;
const pageSize = 50;
const virtualThreshold = 200;
let useVirtualScroll = false;
let allLogs = [];
let auditVirtualList = null;

const debouncedLoad = debounce(loadLogs, 300);

function loadLogs() {
    const search = document.getElementById('search-input').value;
    const action = document.getElementById('action-filter').value;
    const user = document.getElementById('user-filter').value;
    if (useVirtualScroll) { loadAllLogsForVirtualScroll(search, action, user); }
    else { loadLogsPaginated(search, action, user); }
}

function loadLogsPaginated(search, action, user) {
    apiFetch('/audit/logs?page=' + currentPage + '&pageSize=' + pageSize + '&search=' + encodeURIComponent(search) + '&action=' + action + '&user=' + encodeURIComponent(user))
        .then(data => {
            if (data.success) {
                totalRecords = data.total;
                totalPages = Math.ceil(totalRecords / pageSize);
                if (totalRecords > virtualThreshold && !useVirtualScroll) { useVirtualScroll = true; loadAllLogsForVirtualScroll(search, action, user); return; }
                renderLogs(data.data);
                updatePagination();
            } else { showError(data.error || __('Load failed')); }
        }).catch(err => showError(__tf('Network error: {0}', err)));
}

function loadAllLogsForVirtualScroll(search, action, user) {
    apiFetch('/audit/logs?page=1&pageSize=10000&search=' + encodeURIComponent(search) + '&action=' + action + '&user=' + encodeURIComponent(user))
        .then(data => {
            if (data.success) {
                allLogs = data.data || [];
                totalRecords = data.total;
                totalPages = 1;
                initAuditVirtualList();
                updatePagination();
            } else { showError(data.error || __('Load failed')); }
        }).catch(err => showError(__tf('Network error: {0}', err)));
}

function initAuditVirtualList() {
    const container = document.getElementById('audit-table-container');
    if (!container) return;
    if (auditVirtualList) { auditVirtualList.destroy(); auditVirtualList = null; }
    const table = container.querySelector('table');
    if (!table) return;
    const tbody = table.querySelector('tbody');
    if (tbody) { tbody.innerHTML = allLogs.map((log, index) => renderLogRow(log, index)).join(''); }
    auditVirtualList = new VirtualList({ container: container, items: allLogs, threshold: virtualThreshold, buffer: 10, itemHeight: 48, renderItem: function(log, index) { return renderLogRow(log, index); } });
    auditVirtualList.enable();
    const pagination = document.querySelector('.bg-slate-50.border-t.border-slate-200.px-6.py-4');
    if (pagination) pagination.style.display = 'none';
    const totalCount = document.getElementById('total-count');
    if (totalCount) totalCount.textContent = __tf('{0} records (virtual scrolling enabled)', totalRecords);
    setTimeout(() => { if (auditVirtualList) auditVirtualList.recalcHeights(); }, 100);
}

function renderLogRow(log, index) {
    return '<tr class="hover:bg-slate-50 transition-colors"><td class="py-3 px-4 text-sm text-slate-600">' + formatTime(log.created_at) + '</td><td class="py-3 px-4 text-sm"><span class="px-2 py-1 bg-indigo-100 text-indigo-700 rounded-lg text-xs">' + escapeHtml(log.user) + '</span></td><td class="py-3 px-4 text-sm text-slate-600 font-mono">' + escapeHtml(log.ip) + ' || '-'</td><td class="py-3 px-4 text-sm"><span class="px-2 py-1 ' + getActionClass(log.action) + ' rounded-lg text-xs">' + getActionLabel(log.action) + '</span></td><td class="py-3 px-4 text-sm text-slate-600">' + escapeHtml(log.resource) + ' || '-'</td><td class="py-3 px-4 text-sm text-slate-600 font-mono">' + (log.agent_id ? log.agent_id.substring(0, 8) + '...' : '-') + '</td><td class="py-3 px-4 text-sm"><span class="flex items-center">' + (log.success ? '<i class="fa-solid fa-check-circle text-emerald-500 mr-1"></i><span class="text-emerald-600">' + __('Success') + '</span>' : '<i class="fa-solid fa-x-circle text-red-500 mr-1"></i><span class="text-red-600">' + __('Failure') + '</span>') + '</span></td><td class="py-3 px-4 text-sm text-slate-500 max-w-xs truncate" title="' + escapeHtml(log.details || log.error || '-') + '">' + escapeHtml(log.details || log.error || '-') + '</td></tr>';
}

function renderLogs(logs) {
    const tbody = document.getElementById('log-table-body');
    if (logs.length === 0) { tbody.innerHTML = '<tr><td colspan="8" class="py-16 text-center text-slate-400">' + __('No audit records') + '</td></tr>'; return; }
    tbody.innerHTML = logs.map(log => renderLogRow(log)).join('');
}

function formatTime(timeStr) {
    if (!timeStr) return '-';
    const date = new Date(timeStr);
    return date.toLocaleString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function getActionClass(action) {
    const classes = { 'login': 'bg-emerald-100 text-emerald-700', 'logout': 'bg-slate-100 text-slate-700', 'send_command': 'bg-indigo-100 text-indigo-700', 'command_result': 'bg-teal-100 text-teal-700', 'batch_command': 'bg-violet-100 text-violet-700', 'agent_action': 'bg-blue-100 text-blue-700', 'kill_agent': 'bg-red-100 text-red-700', 'delete_agent': 'bg-rose-100 text-rose-700', 'request_screenshot': 'bg-sky-100 text-sky-700', 'request_screenshot_window': 'bg-sky-100 text-sky-700', 'request_ps': 'bg-amber-100 text-amber-700', 'generate': 'bg-purple-100 text-purple-700', 'screen_monitor_start': 'bg-cyan-100 text-cyan-700', 'screen_monitor_stop': 'bg-orange-100 text-orange-700', 'file_ls': 'bg-slate-100 text-slate-700', 'file_delete': 'bg-red-100 text-red-700', 'file_read': 'bg-blue-100 text-blue-700', 'file_upload_exfil': 'bg-emerald-100 text-emerald-700', 'file_download_exfil': 'bg-emerald-100 text-emerald-700' };
    return classes[action] || 'bg-gray-100 text-gray-700';
}

function getActionLabel(action) {
    const labels = { 'login': __('Login'), 'logout': __('Logout'), 'send_command': __('Send Command'), 'command_result': __('Command Result'), 'batch_command': __('Batch Command'), 'agent_action': __('Agent Action'), 'kill_agent': __('Kill Agent'), 'delete_agent': __('Delete Agent'), 'request_screenshot': __('Request Screenshot'), 'request_screenshot_window': __('Request Window Screenshot'), 'request_ps': __('Request Process List'), 'generate': __('Generate'), 'screen_monitor_start': __('Start Monitor'), 'screen_monitor_stop': __('Stop Monitor'), 'file_ls': __('File List'), 'file_delete': __('File Delete'), 'file_read': __('File Read'), 'file_upload_exfil': __('File Upload (Inbound)'), 'file_download_exfil': __('File Download (Exfil)') };
    return labels[action] || action;
}

function updatePagination() {
    document.getElementById('total-count').textContent = __tf('{0} records', totalRecords);
    document.getElementById('current-page').textContent = currentPage;
    document.getElementById('prev-page').disabled = currentPage <= 1;
    document.getElementById('next-page').disabled = currentPage >= totalPages;
    const start = (currentPage - 1) * pageSize + 1;
    const end = Math.min(currentPage * pageSize, totalRecords);
    document.getElementById('page-info').textContent = start + '-' + end;
}

function prevPage() { if (currentPage > 1) { currentPage--; loadLogs(); } }
function nextPage() { if (currentPage < totalPages) { currentPage++; loadLogs(); } }

function applyFilters() {
    currentPage = 1; useVirtualScroll = false; allLogs = [];
    if (auditVirtualList) { auditVirtualList.destroy(); auditVirtualList = null; }
    const pagination = document.querySelector('.bg-slate-50.border-t.border-slate-200.px-6.py-4');
    if (pagination) pagination.style.display = '';
    loadLogs();
}

function resetFilters() {
    document.getElementById('search-input').value = '';
    document.getElementById('action-filter').value = '';
    document.getElementById('user-filter').value = '';
    currentPage = 1; useVirtualScroll = false; allLogs = [];
    if (auditVirtualList) { auditVirtualList.destroy(); auditVirtualList = null; }
    const pagination = document.querySelector('.bg-slate-50.border-t.border-slate-200.px-6.py-4');
    if (pagination) pagination.style.display = '';
    loadLogs();
}

function exportLogs() {
    const search = document.getElementById('search-input').value;
    const action = document.getElementById('action-filter').value;
    const user = document.getElementById('user-filter').value;
    apiFetch('/audit/logs?page=1&pageSize=' + totalRecords + '&search=' + encodeURIComponent(search) + '&action=' + action + '&user=' + encodeURIComponent(user))
        .then(data => {
            if (data.success) { const csv = convertToCSV(data.data); downloadCSV(csv); }
            else { showError(data.error || __('Export failed')); }
        }).catch(err => showError(__tf('Network error: {0}', err)));
}

function convertToCSV(logs) {
    const headers = [__('Time'), __('User'), __('IP'), __('Action'), __('Resource'), __('Agent'), __('Status'), __('Details')];
    const rows = logs.map(log => [formatTime(log.created_at), log.user, log.ip || '', getActionLabel(log.action), log.resource || '', log.agent_id || '', log.success ? __('Success') : __('Failure'), log.details || log.error || '']);
    return [headers.join(','), ...rows.map(row => row.map(cell => '"' + cell.replace(/"/g, '""') + '"').join(','))].join('\n');
}

function downloadCSV(content) {
    const blob = new Blob(['\uFEFF' + content], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = 'audit_logs_' + new Date().toISOString().split('T')[0] + '.csv';
    link.click();
}

function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.appendChild(document.createTextNode(text));
    return div.innerHTML;
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
    setTimeout(() => { toast.style.transition = 'all 0.3s ease'; toast.style.opacity = '0'; setTimeout(() => toast.remove(), 300); }, 3000);
}

(function initAuditPage() {
    const searchInput = document.getElementById('search-input');
    if (!searchInput || !document.getElementById('action-filter')) return;

    searchInput.addEventListener('keyup', function(e) {
        if (e.key === 'Enter') applyFilters();
    });
    searchInput.addEventListener('input', function() {
        debouncedLoad();
    });

    loadLogs();
})();
