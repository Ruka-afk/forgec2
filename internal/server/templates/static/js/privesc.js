function executePrivescCheck() {
    const agentId = document.getElementById('target-agent').value;
    const checkType = document.querySelector('input[name="check_type"]:checked').value;

    if (!agentId) {
        showToast('请选择目标 Implant', 'error');
        return;
    }

    showToast('正在执行提权侦查...', 'success');

    fetch(`/agents/${agentId}/privesc_check`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            check_type: checkType
        })
    })
    .then(r => r.json())
    .then(data => {
        if (data.success) {
            showToast('提权侦查任务已创建', 'success');
        } else {
            showToast('失败: ' + (data.error || ''), 'error');
        }
    })
    .catch(err => {
        console.error('Failed:', err);
        showToast('错误: ' + err, 'error');
    });
}
