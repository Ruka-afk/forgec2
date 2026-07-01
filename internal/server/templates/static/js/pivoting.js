let sessions = [];

async function refreshSessions() {
    try {
        const data = await apiFetch('/socks/sessions');
        sessions = data.sessions || [];
        renderSessions();
    } catch (e) {
        showToast(__tf('Failed to load sessions: {0}', e.message), 'error');
    }
}

function renderSessions() {
    const container = document.getElementById('sessions-container');
    if (sessions.length === 0) {
        container.innerHTML = '<div class="text-slate-400 text-sm py-8 text-center">' + __('No active SOCKS relay sessions') + '</div>';
        return;
    }
    let html = '<div class="space-y-3">';
    for (const s of sessions) {
        const statusColor = s.active ? 'bg-emerald-500' : 'bg-slate-300';
        const statusText = s.active ? __('Running') : __('Stopped');
        const created = s.created_at ? new Date(s.created_at).toLocaleString() : '-';
        html += `
        <div class="border rounded-2xl p-4 ${s.active ? 'border-emerald-200 bg-emerald-50/30' : ''}">
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-x-3">
                    <span class="w-2.5 h-2.5 rounded-full ${statusColor}"></span>
                    <div>
                        <div class="font-medium text-sm">${s.agent_id.substring(0, 12)}...</div>
                        <div class="text-xs text-slate-500">${__tf('Port {0}', s.listen_port)} &middot; ${statusText} &middot; ${__('Created')} ${created}</div>
                    </div>
                </div>
                <div class="flex items-center gap-x-4">
                    <div class="text-xs text-slate-500 text-right">
                        <div><i class="fa-solid fa-arrow-down mr-1"></i>${formatBytes(s.bytes_in || 0)} ${__('in')}</div>
                        <div><i class="fa-solid fa-arrow-up mr-1"></i>${formatBytes(s.bytes_out || 0)} ${__('out')}</div>
                    </div>
                    <div class="text-xs text-slate-500">
                        <i class="fa-solid fa-plug mr-1"></i>${s.active_conn || 0}/${s.conn_count || 0} ${__('connections')}
                    </div>
                    ${s.active ? `
                        <button onclick="stopRelay('${s.agent_id}')" class="px-3 py-1.5 bg-red-100 hover:bg-red-200 text-red-700 rounded-xl text-xs transition-colors">
                            <i class="fa-solid fa-stop mr-1"></i>${__('Stop')}
                        </button>
                    ` : ''}
                </div>
            </div>
        </div>`;
    }
    html += '</div>';
    container.innerHTML = html;
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i];
}

async function startRelay(btn) {
    const agentId = document.getElementById('relay-agent').value;
    const port = document.getElementById('relay-port').value;
    if (!agentId) { showToast(__('Please select an agent'), 'error'); return; }
    if (!port || port < 1 || port > 65535) { showToast(__('Please enter a valid port (1-65535)'), 'error'); return; }
    try {
        btn.disabled = true;
        const data = await apiFetch(`/agents/${agentId}/socks_relay/start`, {
            method: 'POST',
            headers: {'Content-Type': 'application/x-www-form-urlencoded'},
            body: 'port=' + encodeURIComponent(port)
        });
        if (data.success) {
            showToast(__tf('SOCKS relay started on port {0}', data.port), 'success');
            refreshSessions();
        } else {
            showToast(__tf('Failed to start: {0}', data.error || __('Unknown error')), 'error');
        }
    } catch (e) {
        showToast(__tf('Request failed: {0}', e.message), 'error');
    } finally {
        btn.disabled = false;
    }
}

async function stopRelay(agentId) {
    if (!confirm(__('Stop this SOCKS relay? All active connections will be disconnected.'))) return;
    try {
        const data = await apiFetch(`/agents/${agentId}/socks_relay/stop`, {method: 'POST'});
        if (data.success) refreshSessions();
        else showToast(__tf('Failed to stop: {0}', data.error || __('Unknown error')), 'error');
    } catch (e) {
        showToast(__tf('Request failed: {0}', e.message), 'error');
    }
}

function startSocksLocal(agentId) {
    const port = prompt(__('Agent local SOCKS port'), '1080');
    if (!port) return;
    apiFetch(`/agents/${agentId}/socks`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'port=' + encodeURIComponent(port)
    }).then(() => showToast(__('SOCKS task sent to agent'), 'success'))
      .catch(e => showToast(__tf('Failed: {0}', e.message), 'error'));
}

refreshSessions();
setInterval(refreshSessions, 5000);
