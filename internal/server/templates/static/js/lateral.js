function executeLateral() {
    const agentId = document.getElementById('source-agent').value;
    const target = document.getElementById('target-host').value;
    const method = document.querySelector('input[name="method"]:checked').value;
    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;
    const command = document.getElementById('command').value;

    if (!agentId) {
        showToast('请选择源 Implant', 'error');
        return;
    }
    if (!target) {
        showToast('请输入目标主机', 'error');
        return;
    }

    const data = {
        target: target,
        method: method,
        username: username,
        password: password,
        command: command
    };

    showToast('正在执行横向移动...', 'success');

    fetch(`/agents/${agentId}/lateral`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    })
    .then(r => r.json())
    .then(data => {
        if (data.success) {
            showToast('横向移动任务已创建', 'success');
            document.getElementById('target-host').value = '';
        } else {
            showToast('失败: ' + (data.error || ''), 'error');
        }
    })
    .catch(err => {
        console.error('Failed:', err);
        showToast('错误: ' + err, 'error');
    });
}

(function initLateralPage() {
    const credentialEl = document.getElementById('credential');
    if (!credentialEl) return;

    credentialEl.addEventListener('change', function() {
    const option = this.options[this.selectedIndex];
    if (option.value) {
        document.getElementById('username').value = option.dataset.username || '';
        document.getElementById('password').value = '';
        fetch('/credentials/' + option.value)
            .then(r => r.json())
            .then(cred => {
                if (cred.password) document.getElementById('password').value = cred.password;
                else if (cred.hash) document.getElementById('password').value = cred.hash;
            })
            .catch(() => {});
    }
    });
})();
