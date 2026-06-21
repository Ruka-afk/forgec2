// Generate page - P2P toggles, listener selection, all generation forms

document.querySelectorAll('.p2p-mode').forEach(sel => {
    sel.addEventListener('change', function() {
        const card = this.closest('.space-y-4') || this.closest('form') || this.parentElement;
        const parentField = card.querySelector('.p2p-parent-field');
        const listenField = card.querySelector('.p2p-listen-field');
        const dnsFields = card.querySelector('.dns-fields');
        if (this.value === 'child') {
            if (parentField) parentField.classList.remove('hidden');
            if (listenField) listenField.classList.add('hidden');
            if (dnsFields) dnsFields.classList.add('hidden');
        } else if (this.value === 'parent') {
            if (parentField) parentField.classList.add('hidden');
            if (listenField) listenField.classList.remove('hidden');
            if (dnsFields) dnsFields.classList.add('hidden');
        } else if (this.value === 'dns') {
            if (parentField) parentField.classList.add('hidden');
            if (listenField) listenField.classList.add('hidden');
            if (dnsFields) dnsFields.classList.remove('hidden');
        } else {
            if (parentField) parentField.classList.add('hidden');
            if (listenField) listenField.classList.add('hidden');
            if (dnsFields) dnsFields.classList.add('hidden');
        }
    });
});

function onListenerChange(prefix) {
    const sel = document.getElementById('listener-' + prefix);
    if (!sel) return; // shared listener mode
    const opt = sel.options[sel.selectedIndex];
    if (!opt) return;
    fillC2URL(opt, prefix);
}

function onSharedListenerChange() {
    const sel = document.getElementById('shared-listener');
    const opt = sel.options[sel.selectedIndex];
    if (!opt) return;
    // Fill all platform hidden fields
    ['exe', 'ps1', 'linux', 'macos', 'stager', 'stager-linux'].forEach(p => fillC2URL(opt, p));
    // Show C2 URL in shared panel
    const scheme = opt.dataset.scheme;
    const host = opt.dataset.host;
    const port = opt.dataset.port;
    document.getElementById('shared-c2url').value = scheme + '://' + host + ':' + port;
}

function fillC2URL(opt, prefix) {
    const scheme = opt.dataset.scheme;
    const host = opt.dataset.host;
    const port = opt.dataset.port;
    const type = opt.dataset.type;
    let c2url = scheme + '://' + host + ':' + port;
    const failover = document.getElementById('shared-failover');
    if (failover && failover.value.trim()) {
        c2url += ',' + failover.value.trim();
    }
    const c2El = document.getElementById('c2-url-' + prefix);
    if (c2El) c2El.value = c2url;
    let transport = 'http';
    if (scheme === 'tcp' || scheme === 'tls') transport = 'tcp';
    else if (scheme === 'dns' || type === 'dns') transport = 'dns';
    const protoEl = document.getElementById('protocol-' + prefix);
    if (protoEl) protoEl.value = transport;
}

function getSharedSettings() {
    return {
        profile: document.getElementById('shared-profile').value,
        interval: document.getElementById('shared-interval').value,
        jitter: document.getElementById('shared-jitter').value,
        user_agent: document.getElementById('shared-ua').value,
        proxy: document.getElementById('shared-proxy').value,
        crypto_key: document.getElementById('shared-crypto-key').value
    };
}

function getFormData(formId) {
    const form = document.getElementById(formId);
    const formData = new FormData(form);
    const shared = getSharedSettings();
    formData.set('profile', shared.profile);
    formData.set('interval', shared.interval);
    formData.set('jitter', shared.jitter);
    formData.set('user_agent', shared.user_agent);
    formData.set('crypto_key', shared.crypto_key);
    // Use shared listener
    const sharedListener = document.getElementById('shared-listener');
    if (sharedListener && sharedListener.value) {
        formData.set('listener_id', sharedListener.value);
    }
    const persistEl = form.querySelector('input[name="persist"]');
    if (persistEl) formData.set('persist', persistEl.checked ? 'true' : '');
    const skipEl = form.querySelector('input[name="skip_tls_verify"]');
    if (skipEl) formData.set('skip_tls_verify', skipEl.checked ? 'true' : '');
    return formData;
}

async function generateEXE() {
    const sharedListener = document.getElementById('shared-listener');
    if (!sharedListener || !sharedListener.value) { showToast('请先在公共设置中选择监听器', 'error'); return; }
    const formData = getFormData('exe-form');
    const btn = document.querySelector('#exe-form button');
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 正在生成...';
    document.getElementById('exe-result').innerHTML = '';
    try {
        const response = await fetch('/generate/exe', {
            method: 'POST', headers: { 'X-CSRF-Token': getCSRFToken() }, body: formData
        });
        if (response.ok) {
            const blob = await response.blob();
            const contentDisposition = response.headers.get('Content-Disposition');
            let filename = 'forge_agent.exe';
            if (contentDisposition) { const match = contentDisposition.match(/filename=(.+)/); if (match) filename = match[1].replace(/"/g, ''); }
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url; a.download = filename; document.body.appendChild(a); a.click();
            window.URL.revokeObjectURL(url); a.remove();
            showToast('EXE 生成并下载成功！', 'success');
        } else {
            const text = await response.text();
            document.getElementById('exe-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + text + '</div>';
        }
    } catch (err) {
        document.getElementById('exe-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + err.message + '</div>';
    } finally {
        btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-download"></i> 生成 EXE';
    }
}

async function generatePS1() {
    const sharedListener = document.getElementById('shared-listener');
    if (!sharedListener || !sharedListener.value) { showToast('请先在公共设置中选择监听器', 'error'); return; }
    const formData = getFormData('ps1-form');
    const btn = document.querySelector('#ps1-form button');
    btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 正在生成...';
    document.getElementById('ps1-result').innerHTML = '';
    try {
        const response = await fetch('/generate/ps1', {
            method: 'POST', headers: { 'X-CSRF-Token': getCSRFToken() }, body: formData
        });
        if (response.ok) {
            const data = await response.json();
            if (data.success) {
                document.getElementById('ps1-result').innerHTML = `
                    <div class="mt-3">
                        <div class="flex items-center justify-between mb-2">
                            <span class="text-xs text-emerald-600 font-medium"><i class="fa-solid fa-check-circle"></i> 生成成功！原始: ${data.original_length} B → 混淆: ${data.obfuscated_len} B</span>
                            <button onclick="copyToClipboard()" class="text-xs px-3 py-1.5 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl flex items-center gap-x-1"><i class="fa-solid fa-copy"></i> 复制</button>
                        </div>
                        <textarea id="ps1-code" readonly class="w-full h-48 bg-slate-900 text-emerald-400 font-mono text-xs rounded-xl p-3 border border-slate-700 resize-none">${escapeHtml(data.code)}</textarea>
                        <div class="mt-1 text-xs text-slate-500"><i class="fa-solid fa-info-circle"></i> 直接在 PowerShell 中粘贴执行</div>
                    </div>`;
            }
        } else {
            const text = await response.text();
            document.getElementById('ps1-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + text + '</div>';
        }
    } catch (err) {
        document.getElementById('ps1-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + err.message + '</div>';
    } finally {
        btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-magic"></i> 生成 PS1';
    }
}

async function generateStager() {
    const sharedListener = document.getElementById('shared-listener');
    if (!sharedListener || !sharedListener.value) { showToast('请先在公共设置中选择监听器', 'error'); return; }
    const form = document.getElementById('stager-form');
    const formData = new FormData(form);
    const shared = getSharedSettings();
    formData.set('profile', shared.profile); formData.set('interval', shared.interval);
    formData.set('jitter', shared.jitter); formData.set('user_agent', shared.user_agent);
    formData.set('proxy', shared.proxy);
    formData.set('listener_id', sharedListener.value);
    const skipEl = form.querySelector('input[name="skip_tls_verify"]');
    if (skipEl) formData.set('skip_tls_verify', skipEl.checked ? 'true' : '');
    const btn = form.querySelector('button'); btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 正在生成...';
    document.getElementById('stager-result').innerHTML = '';
    try {
        const response = await fetch('/generate/stager', {
            method: 'POST', headers: { 'X-CSRF-Token': getCSRFToken() }, body: formData
        });
        if (response.ok) {
            const blob = await response.blob();
            const cd = response.headers.get('Content-Disposition');
            let fn = 'stager.exe'; if (cd) { const m = cd.match(/filename=(.+)/); if (m) fn = m[1].replace(/"/g,''); }
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a'); a.href = url; a.download = fn;
            document.body.appendChild(a); a.click(); window.URL.revokeObjectURL(url); a.remove();
            showToast('加载器 + 阶段负载生成成功！', 'success');
        } else {
            const text = await response.text();
            document.getElementById('stager-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + text + '</div>';
        }
    } catch (err) {
        document.getElementById('stager-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + err.message + '</div>';
    } finally {
        btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-download"></i> 生成加载器 + 阶段负载';
    }
}

async function generateStagerLinux() {
    const sharedListener = document.getElementById('shared-listener');
    if (!sharedListener || !sharedListener.value) { showToast('请先在公共设置中选择监听器', 'error'); return; }
    const form = document.getElementById('stager-linux-form');
    const formData = new FormData(form);
    const shared = getSharedSettings();
    formData.set('profile', shared.profile); formData.set('interval', shared.interval);
    formData.set('jitter', shared.jitter); formData.set('user_agent', shared.user_agent);
    formData.set('proxy', shared.proxy);
    formData.set('listener_id', sharedListener.value);
    const skipEl = form.querySelector('input[name="skip_tls_verify"]');
    if (skipEl) formData.set('skip_tls_verify', skipEl.checked ? 'true' : '');
    const btn = form.querySelector('button'); btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 正在生成...';
    document.getElementById('stager-linux-result').innerHTML = '';
    try {
        const response = await fetch('/generate/stager_linux', {
            method: 'POST', headers: { 'X-CSRF-Token': getCSRFToken() }, body: formData
        });
        if (response.ok) {
            const blob = await response.blob();
            const cd = response.headers.get('Content-Disposition');
            let fn = 'stager'; if (cd) { const m = cd.match(/filename=(.+)/); if (m) fn = m[1].replace(/"/g,''); }
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a'); a.href = url; a.download = fn;
            document.body.appendChild(a); a.click(); window.URL.revokeObjectURL(url); a.remove();
            showToast('Linux 加载器 + 阶段负载生成成功！', 'success');
        } else {
            const text = await response.text();
            document.getElementById('stager-linux-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + text + '</div>';
        }
    } catch (err) {
        document.getElementById('stager-linux-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + err.message + '</div>';
    } finally {
        btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-download"></i> 生成加载器 + 阶段负载';
    }
}

async function generateLinux() {
    const sharedListener = document.getElementById('shared-listener');
    if (!sharedListener || !sharedListener.value) { showToast('请先在公共设置中选择监听器', 'error'); return; }
    const formData = getFormData('linux-form');
    const btn = document.querySelector('#linux-form button');
    btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 正在生成...';
    try {
        const response = await fetch('/generate/linux', {
            method: 'POST', headers: { 'X-CSRF-Token': getCSRFToken() }, body: formData
        });
        if (response.ok) {
            const blob = await response.blob();
            const cd = response.headers.get('Content-Disposition');
            let fn = 'forgec2_agent'; if (cd) { const m = cd.match(/filename=(.+)/); if (m) fn = m[1].replace(/"/g,''); }
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a'); a.href = url; a.download = fn;
            document.body.appendChild(a); a.click(); window.URL.revokeObjectURL(url); a.remove();
            showToast('Linux ELF 生成并下载成功！', 'success');
        } else {
            const text = await response.text();
            document.getElementById('linux-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + text + '</div>';
        }
    } catch (err) {
        document.getElementById('linux-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + err.message + '</div>';
    } finally {
        btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-download"></i> 生成 ELF';
    }
}

async function generateMacOS() {
    const sharedListener = document.getElementById('shared-listener');
    if (!sharedListener || !sharedListener.value) { showToast('请先在公共设置中选择监听器', 'error'); return; }
    const formData = getFormData('macos-form');
    const btn = document.querySelector('#macos-form button');
    btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 正在生成...';
    document.getElementById('macos-result').innerHTML = '';
    try {
        const response = await fetch('/generate/macos', {
            method: 'POST', headers: { 'X-CSRF-Token': getCSRFToken() }, body: formData
        });
        if (response.ok) {
            const blob = await response.blob();
            const cd = response.headers.get('Content-Disposition');
            let fn = 'forgec2_agent'; if (cd) { const m = cd.match(/filename=(.+)/); if (m) fn = m[1].replace(/"/g,''); }
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a'); a.href = url; a.download = fn;
            document.body.appendChild(a); a.click(); window.URL.revokeObjectURL(url); a.remove();
            showToast('macOS Binary 生成并下载成功！', 'success');
        } else {
            const text = await response.text();
            document.getElementById('macos-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + text + '</div>';
        }
    } catch (err) {
        document.getElementById('macos-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + err.message + '</div>';
    } finally {
        btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-download"></i> 生成 macOS Binary';
    }
}

async function generateOneLiner() {
    const listenerSel = document.getElementById('listener-oneliner');
    if (!listenerSel.value) { showToast('请先选择一个监听器', 'error'); return; }
    const form = document.getElementById('oneliner-form');
    const formData = new FormData(form);
    const shared = getSharedSettings();
    formData.set('profile', shared.profile); formData.set('interval', shared.interval);
    formData.set('jitter', shared.jitter); formData.set('user_agent', shared.user_agent);
    formData.set('proxy', shared.proxy);
    const persistEl = form.querySelector('input[name="persist"]');
    if (persistEl) formData.set('persist', persistEl.checked ? 'true' : '');
    const skipEl = form.querySelector('input[name="skip_tls_verify"]');
    if (skipEl) formData.set('skip_tls_verify', skipEl.checked ? 'true' : '');
    const btn = form.querySelector('button'); btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 正在生成...';
    document.getElementById('oneliner-result').innerHTML = '';
    try {
        const response = await fetch('/generate/one-liner', {
            method: 'POST', headers: { 'X-CSRF-Token': getCSRFToken() }, body: formData
        });
        const result = await response.json();
        if (!result.success) { document.getElementById('oneliner-result').innerHTML = '<div class="text-sm text-red-500">错误: ' + escapeHtml(result.error || '') + '</div>'; return; }
        let html = '<div class="text-xs text-emerald-600 mb-3 flex items-center gap-x-2"><i class="fa-solid fa-check-circle"></i>负载已托管: <code class="text-xs bg-slate-100 px-2 py-0.5 rounded">' + escapeHtml(result.download_url) + '</code>（1小时有效）</div>';
        html += '<div class="space-y-2">';
        for (const item of result.types) {
            html += '<div class="border border-slate-200 rounded-2xl p-3 hover:border-rose-200 transition-colors">';
            html += '<div class="flex items-center justify-between mb-1.5">';
            html += '<div><span class="text-sm font-medium text-slate-800">' + escapeHtml(item.name) + '</span><span class="text-[10px] text-slate-400 ml-2">' + escapeHtml(item.desc) + '</span></div>';
            html += '<button onclick="copyOneLiner(this)" class="text-xs px-2.5 py-1 bg-slate-100 hover:bg-rose-100 rounded-xl text-slate-600"><i class="fa-regular fa-copy mr-1"></i>复制</button>';
            html += '</div>';
            html += '<code class="block text-[11px] font-mono bg-slate-50 text-slate-700 p-2 rounded-xl whitespace-pre-wrap break-all leading-relaxed select-all">' + escapeHtml(item.command) + '</code>';
            html += '</div>';
        }
        html += '</div>';
        document.getElementById('oneliner-result').innerHTML = html;
        showToast('生成 ' + result.types.length + ' 条 One-Liner 命令', 'success');
    } catch (err) {
        document.getElementById('oneliner-result').innerHTML = '<div class="text-sm text-red-500">请求失败: ' + escapeHtml(err.message) + '</div>';
    } finally {
        btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-bolt"></i> 生成 One-Liner 命令';
    }
}

function copyOneLiner(btn) {
    const code = btn.closest('.border').querySelector('code').textContent;
    navigator.clipboard.writeText(code).then(() => showToast('命令已复制！', 'success'))
    .catch(() => { const ta = document.createElement('textarea'); ta.value = code; ta.style.position = 'fixed'; ta.style.opacity = '0'; document.body.appendChild(ta); ta.select(); document.execCommand('copy'); ta.remove(); showToast('命令已复制！', 'success'); });
}

function copyToClipboard() {
    const code = document.getElementById('ps1-code').value;
    navigator.clipboard.writeText(code).then(() => showToast('代码已复制到剪贴板！', 'success'))
    .catch(() => { document.getElementById('ps1-code').select(); document.execCommand('copy'); showToast('代码已复制！', 'success'); });
}

async function createListener() {
    const name = prompt("监听器名称:", "My DNS Listener");
    if (!name) return;
    const ltype = prompt("类型 (http/tcp/dns):", "dns");
    const host = prompt("Domain/IP:", "c2.example.com");
    const port = ltype === 'dns' ? 53 : parseInt(prompt("Port:", "8080") || "8080");
    const proto = ltype === 'dns' ? 'dns' : prompt("协议 (http/https/tcp/tls):", ltype);
    const res = await fetch('/listeners', {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': getCSRFToken() },
        body: JSON.stringify({ name, type: ltype, host, port, protocol: proto, enabled: true })
    });
    const data = await res.json();
    if (data.success) { showToast('监听器创建成功，请刷新页面', 'success'); location.reload(); }
    else { showToast('创建失败: ' + (data.error || ''), 'error'); }
}
