function showAddTemplateModal() {
    document.getElementById('add-template-modal').classList.remove('hidden');
}

function hideAddTemplateModal() {
    document.getElementById('add-template-modal').classList.add('hidden');
}

function saveTemplate() {
    const data = {
        name: document.getElementById('template-name').value,
        category: document.getElementById('template-category').value,
        command: document.getElementById('template-command').value,
        description: document.getElementById('template-description').value
    };

    if (!data.name || !data.command) {
        showToast('请填写名称和命令', 'error');
        return;
    }

    fetch('/api/templates', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    })
    .then(r => r.json())
    .then(result => {
        if (result.success) {
            showToast('模板已保存', 'success');
            location.reload();
        } else {
            showToast('失败: ' + (result.error || ''), 'error');
        }
    })
    .catch(err => {
        console.error('Failed:', err);
        showToast('错误: ' + err, 'error');
    });
}

function deleteTemplate(id) {
    if (!confirm('确认删除此模板？')) return;

    fetch(`/api/templates/${id}`, {
        method: 'DELETE'
    })
    .then(r => r.json())
    .then(result => {
        if (result.success) {
            showToast('模板已删除', 'success');
            location.reload();
        } else {
            showToast('失败: ' + (result.error || ''), 'error');
        }
    })
    .catch(err => {
        console.error('Failed:', err);
        showToast('错误: ' + err, 'error');
    });
}

function useTemplate(command) {
    // Copy to clipboard
    navigator.clipboard.writeText(command).then(() => {
        showToast('命令已复制到剪贴板', 'success');
    }).catch(() => {
        showToast('复制失败，请手动选择', 'error');
    });
}
