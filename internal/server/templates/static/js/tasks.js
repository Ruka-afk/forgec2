// Auto-refresh: poll for pending tasks
let refreshTimer = null;
let tasksVirtualList = null;

function initTasksVirtualList() {
    const container = document.getElementById('tasks-table-container');
    if (!container) return;
    
    const tbody = container.querySelector('tbody');
    if (!tbody) return;
    
    const rows = tbody.querySelectorAll('tr');
    if (rows.length > 200) {
        tasksVirtualList = new VirtualList({
            container: container,
            threshold: 200,
            buffer: 10,
            itemHeight: 65
        });
        tasksVirtualList.enable();
        
        setTimeout(() => {
            if (tasksVirtualList) {
                tasksVirtualList.recalcHeights();
            }
        }, 100);
    }
}

function autoRefresh() {
    const pending = document.querySelectorAll('.task-row[data-status="pending"]');
    if (pending.length > 0) {
        document.getElementById('live-status').classList.remove('hidden');
        fetch(window.location.href, { headers: { 'X-Requested-With': 'XMLHttpRequest' } })
            .then(r => r.text())
            .then(html => {
                const parser = new DOMParser();
                const doc = parser.parseFromString(html, 'text/html');
                const newTbody = doc.getElementById('tasks-tbody');
                const oldTbody = document.getElementById('tasks-tbody');
                if (newTbody && oldTbody) {
                    oldTbody.innerHTML = newTbody.innerHTML;
                    if (tasksVirtualList && tasksVirtualList.enabled) {
                        tasksVirtualList.disable();
                        tasksVirtualList = null;
                        initTasksVirtualList();
                    }
                }
                const stillPending = document.querySelectorAll('.task-row[data-status="pending"]');
                if (stillPending.length === 0) {
                    document.getElementById('live-status').classList.add('hidden');
                    if (refreshTimer) { clearTimeout(refreshTimer); refreshTimer = null; }
                } else {
                    refreshTimer = setTimeout(autoRefresh, 5000);
                }
            })
            .catch(() => {
                refreshTimer = setTimeout(autoRefresh, 5000);
            });
    } else {
        document.getElementById('live-status').classList.add('hidden');
        if (refreshTimer) { clearTimeout(refreshTimer); refreshTimer = null; }
    }
}

// Start auto-refresh on load
document.addEventListener('DOMContentLoaded', function() {
    initTasksVirtualList();
    const pending = document.querySelectorAll('.task-row[data-status="pending"]');
    if (pending.length > 0) {
        refreshTimer = setTimeout(autoRefresh, 5000);
    }
});

function exportTasks() {
    const a = document.createElement('a');
    a.href = '/tasks/export';
    a.download = 'forgec2_tasks_' + new Date().toISOString().split('T')[0] + '.csv';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    showToast && showToast('正在导出任务报告...');
}

function rerunTask(agentId, taskId) {
    fetch(`/agents/${agentId}/task/${taskId}/rerun`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                showToast('任务已重新创建，等待 Implant 执行');
                setTimeout(() => location.reload(), 1500);
            } else {
                showToast('重跑失败: ' + (data.error || '未知错误'), 'error');
            }
        })
        .catch(err => showToast('请求失败: ' + err, 'error'));
}

function retryAllFailed() {
    if (!confirm('确认要重试所有失败任务？')) return;
    const rows = document.querySelectorAll('.task-row[data-status="failed"]');
    let count = 0;
    rows.forEach(row => {
        const agentId = row.dataset.agentId;
        const taskId = row.dataset.taskId;
        if (agentId && taskId) {
            fetch(`/agents/${agentId}/task/${taskId}/rerun`, { method: 'POST' })
                .then(r => r.json())
                .then(data => { if (data.success) count++; });
        }
    });
    showToast(`正在重试 ${rows.length} 个失败任务...`);
    setTimeout(() => location.reload(), 2000);
}

// Keyboard shortcut: Ctrl+F to focus search
document.addEventListener('keydown', function(e) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
        const searchInput = document.querySelector('input[name="q"]');
        if (searchInput && !e.target.closest('input, textarea')) {
            e.preventDefault();
            searchInput.focus();
        }
    }
});
