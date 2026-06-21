function copyCredential(id) {
    fetch('/credentials/' + id)
        .then(r => r.json())
        .then(cred => {
            if (cred.password) {
                navigator.clipboard.writeText(cred.password).then(() => showToast('密码已复制'));
            } else {
                showToast('无密码可复制', 'error');
            }
        }).catch(() => showToast('获取凭据失败', 'error'));
}

function showAddCredModal() {
    document.getElementById('add-cred-modal').classList.remove('hidden');
}
function hideAddCredModal() {
    document.getElementById('add-cred-modal').classList.add('hidden');
}

document.getElementById('add-cred-form').addEventListener('submit', function(e) {
    e.preventDefault();
    const fd = new FormData(this);
    fetch('/credentials/add', {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: new URLSearchParams(fd).toString()
    }).then(r => r.json()).then(data => {
        if (data.success) {
            hideAddCredModal();
            showToast('凭据已添加');
            setTimeout(() => location.reload(), 800);
        } else {
            showToast('添加失败: ' + (data.error || ''), 'error');
        }
    });
});

function deleteCredential(id) {
    if (!confirm('确定删除此凭据？')) return;
    fetch('/credentials/' + id, { method: 'DELETE' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                showToast('已删除');
                setTimeout(() => location.reload(), 500);
            }
        });
}
