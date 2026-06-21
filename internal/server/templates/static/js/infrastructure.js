// Infrastructure page - redirector config generation, ACME provisioning, profile export

function getInfraSettings() {
    const listenerEl = document.getElementById('infra-listener');
    const selected = listenerEl.options[listenerEl.selectedIndex];
    const backendHost = selected ? selected.getAttribute('data-host') : '';
    const backendPort = selected ? selected.getAttribute('data-port') : '';
    const backendProto = selected ? selected.getAttribute('data-protocol') : 'http';
    const defaultDomain = backendHost || 'c2.example.com';

    const domain = document.getElementById('infra-domain').value || defaultDomain;
    const port = document.getElementById('infra-port').value || 443;
    const certPath = document.getElementById('infra-cert').value || ('/etc/letsencrypt/live/' + domain + '/fullchain.pem');
    const keyPath = document.getElementById('infra-key').value || ('/etc/letsencrypt/live/' + domain + '/privkey.pem');
    const wsEnabled = document.getElementById('infra-ws').checked;
    const extc2Enabled = document.getElementById('infra-extc2').checked;
    const backendURL = (backendProto === 'https' ? 'https' : 'http') + '://' + (backendHost || '127.0.0.1') + ':' + (backendPort || '8080');

    return {
        domain: domain,
        listen_port: parseInt(port),
        backend_url: backendURL,
        cert_path: certPath,
        key_path: keyPath,
        ws_enabled: wsEnabled,
        extc2_paths: extc2Enabled ? ['/extc2/v1/receive', '/extc2/v1/send'] : [],
        blocked_ips: [],
        user_agent: '',
        profile: ''
    };
}

function onInfraListenerChange() {
    const el = document.getElementById('infra-listener');
    const info = document.getElementById('infra-listener-info');
    const selected = el.options[el.selectedIndex];
    if (selected && selected.value) {
        const host = selected.getAttribute('data-host');
        const port = selected.getAttribute('data-port');
        const proto = selected.getAttribute('data-protocol');
        info.querySelector('span').textContent = '后端: ' + proto + '://' + host + ':' + port;
        info.classList.remove('hidden');
        if (!document.getElementById('infra-domain').value) {
            document.getElementById('infra-domain').value = host;
        }
    } else {
        info.classList.add('hidden');
    }
}

function generateInfra(type) {
    const settings = getInfraSettings();

    fetch('/infrastructure/generate/' + type, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings)
    })
    .then(r => r.json())
    .then(data => {
        if (data.config) {
            document.getElementById('infra-config-output').textContent = data.config;
            document.getElementById('config-type-label').textContent = type.charAt(0).toUpperCase() + type.slice(1);
            document.getElementById('infra-result').classList.remove('hidden');
        } else {
            showToast(data.error || '生成失败', 'error');
        }
    })
    .catch(err => showToast('请求失败: ' + err, 'error'));
}

function copyInfraConfig() {
    const text = document.getElementById('infra-config-output').textContent;
    navigator.clipboard.writeText(text).then(() => {
        showToast('配置已复制到剪贴板', 'success');
    }).catch(() => showToast('复制失败', 'error'));
}

function provisionACME() {
    const domain = document.getElementById('acme-domain').value;
    const email = document.getElementById('acme-email').value;
    const port = parseInt(document.getElementById('acme-port').value) || 80;
    const useStaging = document.getElementById('acme-staging').checked;

    if (!domain || !email) {
        showToast('请填写域名和邮箱', 'error');
        return;
    }

    const btn = event.target;
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 签发中...';

    fetch('/infrastructure/acme/provision', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain: domain, email: email, port: port, use_staging: useStaging })
    })
    .then(r => r.json())
    .then(data => {
        if (data.success) {
            document.getElementById('acme-output').textContent =
                '证书已签发!\n\n证书路径: ' + (data.cert_file || 'N/A') +
                '\n密钥路径: ' + (data.key_file || 'N/A') +
                '\n过期时间: ' + (data.expires || 'N/A') +
                '\n\n请将上述路径填入 Step 2 的 SSL 证书/密钥路径';
            document.getElementById('acme-result').classList.remove('hidden');
            showToast('证书签发成功!', 'success');
            if (data.cert_file) {
                document.getElementById('infra-cert').value = data.cert_file.replace(/\\/g, '/');
            }
            if (data.key_file) {
                document.getElementById('infra-key').value = data.key_file.replace(/\\/g, '/');
            }
        } else {
            showToast('签发失败: ' + (data.error || '未知错误'), 'error');
            document.getElementById('acme-output').textContent = '错误: ' + (data.error || '未知错误');
            document.getElementById('acme-result').classList.remove('hidden');
        }
    })
    .catch(err => showToast('请求失败: ' + err, 'error'))
    .finally(() => {
        btn.disabled = false;
        btn.innerHTML = '<i class="fa-solid fa-certificate"></i> 自动签发证书';
    });
}

function exportProfile(format) {
    fetch('/infrastructure/profile/export?format=' + format)
    .then(r => r.json())
    .then(data => {
        if (data.content) {
            document.getElementById('export-output').textContent = data.content;
            document.getElementById('export-result').classList.remove('hidden');
        } else {
            showToast(data.error || '导出失败', 'error');
        }
    })
    .catch(err => showToast('请求失败: ' + err, 'error'));
}

function copyExportConfig() {
    const text = document.getElementById('export-output').textContent;
    navigator.clipboard.writeText(text).then(() => {
        showToast('已复制到剪贴板', 'success');
    }).catch(() => showToast('复制失败', 'error'));
}
