let credentialsVirtualList = null;

function initCredentialsVirtualList() {
    const container = document.getElementById('credentials-table-container');
    if (!container) return;
    
    const tbody = container.querySelector('tbody');
    if (!tbody) return;
    
    const rows = tbody.querySelectorAll('tr');
    if (rows.length > 200) {
        credentialsVirtualList = new VirtualList({
            container: container,
            threshold: 200,
            buffer: 10,
            itemHeight: 55
        });
        credentialsVirtualList.enable();
        
        setTimeout(() => {
            if (credentialsVirtualList) {
                credentialsVirtualList.recalcHeights();
            }
        }, 100);
    }
}

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
    document.getElementById('add-cred-form').reset();
}

(function initCredentialsPage() {
    if (!document.getElementById('add-cred-form')) return;

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

function editCredential(id) {
    fetch('/credentials/' + id)
        .then(r => r.json())
        .then(cred => {
            document.getElementById('edit-cred-id').value = cred.id;
            document.getElementById('edit-domain').value = cred.domain || '';
            document.getElementById('edit-username').value = cred.username || '';
            document.getElementById('edit-tags').value = cred.tags || '';
            document.getElementById('edit-notes').value = cred.notes || '';
            if (cred.expires_at) {
                const date = new Date(cred.expires_at);
                document.getElementById('edit-expires_at').value = date.toISOString().split('T')[0];
            }
            document.getElementById('edit-cred-modal').classList.remove('hidden');
        }).catch(() => showToast('获取凭据失败', 'error'));
}

function hideEditCredModal() {
    document.getElementById('edit-cred-modal').classList.add('hidden');
}

document.getElementById('edit-cred-form').addEventListener('submit', function(e) {
    e.preventDefault();
    const id = document.getElementById('edit-cred-id').value;
    const fd = new FormData(this);
    fetch('/credentials/' + id, {
        method: 'PUT',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: new URLSearchParams(fd).toString()
    }).then(r => r.json()).then(data => {
        if (data.success) {
            hideEditCredModal();
            showToast('凭据已更新');
            setTimeout(() => location.reload(), 800);
        } else {
            showToast('更新失败: ' + (data.error || ''), 'error');
        }
    });
});

function toggleConfirmed(id) {
    fetch('/credentials/' + id + '/confirm', { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                const status = data.confirmed ? '已验证' : '未验证';
                showToast('凭据标记为: ' + status);
                setTimeout(() => location.reload(), 500);
            } else {
                showToast('操作失败', 'error');
            }
        });
}

function toggleSelectAll() {
    const checkboxes = document.querySelectorAll('.cred-checkbox');
    const selectAll = document.getElementById('select-all');
    checkboxes.forEach(cb => cb.checked = selectAll.checked);
}

function getSelectedIds() {
    const checkboxes = document.querySelectorAll('.cred-checkbox:checked');
    const ids = [];
    checkboxes.forEach(cb => ids.push(parseInt(cb.dataset.id)));
    return ids;
}

function showBatchTagsModal() {
    const selected = getSelectedIds();
    if (selected.length === 0) {
        showToast('请先选择凭据', 'error');
        return;
    }
    document.getElementById('batch-tags-modal').classList.remove('hidden');
}

function hideBatchTagsModal() {
    document.getElementById('batch-tags-modal').classList.add('hidden');
    document.getElementById('batch-tags-input').value = '';
}

document.getElementById('batch-tags-form').addEventListener('submit', function(e) {
    e.preventDefault();
    const tagsInput = document.getElementById('batch-tags-input').value;
    const tags = tagsInput.split(/[,\n]/).map(t => t.trim()).filter(t => t !== '');
    
    if (tags.length === 0) {
        showToast('请输入标签', 'error');
        return;
    }
    
    const ids = getSelectedIds();
    if (ids.length === 0) {
        showToast('请选择凭据', 'error');
        return;
    }
    
    fetch('/credentials/batch/tags', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({ ids, tags })
    }).then(r => r.json()).then(data => {
        if (data.success) {
            hideBatchTagsModal();
            showToast('标签已添加到 ' + data.count + ' 条凭据');
            setTimeout(() => location.reload(), 800);
        } else {
            showToast('操作失败: ' + (data.error || ''), 'error');
        }
    });
});

function filterByTag(tag) {
    const currentParams = new URLSearchParams(window.location.search);
    if (currentParams.get('tag') === tag) {
        currentParams.delete('tag');
    } else {
        currentParams.set('tag', tag);
    }
    window.location.search = currentParams.toString();
}

function applyFilters() {
    const search = document.getElementById('search-input').value;
    const expiry = document.getElementById('expiry-filter').value;
    const confirmed = document.getElementById('confirmed-filter').value;
    
    const params = new URLSearchParams();
    if (search) params.set('search', search);
    if (expiry) params.set('expiry', expiry);
    if (confirmed) params.set('confirmed', confirmed);
    
    window.location.search = params.toString();
}

function clearFilters() {
    window.location.search = '';
}

function showExportOptions() {
    document.getElementById('export-options-modal').classList.remove('hidden');
}

function hideExportOptions() {
    document.getElementById('export-options-modal').classList.add('hidden');
}

function exportCredentials() {
    const scope = document.querySelector('input[name="export_scope"]:checked').value;
    const currentParams = new URLSearchParams(window.location.search);
    const urlParams = new URLSearchParams();
    
    if (scope === 'expired') {
        urlParams.set('expiry', 'expired');
    } else if (scope === 'current') {
        const tag = currentParams.get('tag');
        const expiry = currentParams.get('expiry');
        const search = currentParams.get('search');
        const confirmed = currentParams.get('confirmed');
        if (tag) urlParams.set('tag', tag);
        if (expiry) urlParams.set('expiry', expiry);
        if (search) urlParams.set('search', search);
        if (confirmed) urlParams.set('confirmed', confirmed);
    }
    
    window.location.href = '/credentials/export?' + urlParams.toString();
    hideExportOptions();
}

document.getElementById('search-input').addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
        applyFilters();
    }
});

document.addEventListener('DOMContentLoaded', function() {
    initCredentialsVirtualList();
});
})();