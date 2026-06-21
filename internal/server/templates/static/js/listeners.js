let allListeners = [];

function filterListeners() {
    const search = document.getElementById('search-input').value.toLowerCase();
    const type = document.getElementById('type-filter').value;
    const status = document.getElementById('status-filter').value;

    const rows = document.querySelectorAll('.listener-row');
    rows.forEach(row => {
        const name = row.dataset.name.toLowerCase();
        const t = row.dataset.type;
        const enabled = row.dataset.enabled === 'true';

        let show = true;
        if (search && !name.includes(search)) show = false;
        if (type && t !== type) show = false;
        if (status === 'enabled' && !enabled) show = false;
        if (status === 'disabled' && enabled) show = false;

        row.style.display = show ? '' : 'none';
    });
}

function showCreateModal() {
    document.getElementById('modal-title').textContent = '创建监听器';
    document.getElementById('listener-id').value = '';
    document.getElementById('listener-name').value = '';
    document.getElementById('listener-scheme').value = 'http';
    document.getElementById('listener-host').value = '0.0.0.0';
    document.getElementById('listener-port').value = '8080';
    document.getElementById('listener-notes').value = '';
    document.getElementById('listener-enabled').checked = true;
    document.getElementById('listener-modal').classList.remove('hidden');
    document.getElementById('listener-modal').classList.add('flex');
}

function editListener(id, name, scheme, host, port, notes, enabled) {
    document.getElementById('modal-title').textContent = '编辑监听器';
    document.getElementById('listener-id').value = id;
    document.getElementById('listener-name').value = name;
    document.getElementById('listener-scheme').value = scheme || 'http';
    document.getElementById('listener-host').value = host;
    document.getElementById('listener-port').value = port;
    document.getElementById('listener-notes').value = notes || '';
    document.getElementById('listener-enabled').checked = enabled;
    document.getElementById('listener-modal').classList.remove('hidden');
    document.getElementById('listener-modal').classList.add('flex');
}

function hideListenerModal() {
    const modal = document.getElementById('listener-modal');
    modal.classList.remove('flex');
    modal.classList.add('hidden');
}

async function saveListener() {
    const id = document.getElementById('listener-id').value;
    const scheme = document.getElementById('listener-scheme').value;

    const payload = {
        name: document.getElementById('listener-name').value,
        scheme: scheme,
        host: document.getElementById('listener-host').value,
        port: parseInt(document.getElementById('listener-port').value),
        notes: document.getElementById('listener-notes').value,
        enabled: document.getElementById('listener-enabled').checked
    };

    if (scheme === 'http' || scheme === 'https') {
        payload.type = 'http';
        payload.protocol = scheme;
    } else {
        payload.type = 'tcp';
        payload.protocol = scheme;
    }

    const url = id ? `/api/listeners/${id}` : '/api/listeners';
    const method = id ? 'PUT' : 'POST';

    const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': getCSRFToken() },
        body: JSON.stringify(payload)
    });

    const data = await res.json();
    if (data.success) {
        hideListenerModal();
        location.reload();
    } else {
        showToast('保存失败: ' + (data.error || '未知错误'), 'error');
    }
}

function toggleListener(id, currentEnabled) {
    fetch(`/api/listeners/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': getCSRFToken() },
        body: JSON.stringify({ enabled: !currentEnabled })
    }).then(() => location.reload());
}

function deleteListener(id, name) {
    if (!confirm(`确定删除监听器 "${name}" 吗？`)) return;
    fetch(`/api/listeners/${id}`, {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': getCSRFToken() }
    }).then(() => location.reload());
}

function copyConnect(url, name) {
    navigator.clipboard.writeText(url).then(() => {
        const toast = document.createElement('div');
        toast.className = 'fixed bottom-6 left-1/2 -translate-x-1/2 bg-emerald-600 text-white px-6 py-2 rounded-3xl text-sm shadow-lg';
        toast.textContent = `已复制 ${name} 的连接地址`;
        document.body.appendChild(toast);
        setTimeout(() => toast.remove(), 1800);
    });
}
