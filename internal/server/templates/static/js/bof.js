// BOF management page - upload, run, download, delete, edit, results

function showUploadModal() { document.getElementById('upload-modal').classList.remove('hidden'); }

function closeModal(id) { document.getElementById(id).classList.add('hidden'); }

function uploadBOF(e) {
    e.preventDefault();
    const form = document.getElementById('upload-form');
    const fd = new FormData(form);
    fetch('/api/bof/upload', { method: 'POST', body: fd })
        .then(r => r.json())
        .then(d => {
            if (d.success) {
                showToast('BOF 上传成功', 'success');
                closeModal('upload-modal');
                setTimeout(() => location.reload(), 800);
            } else {
                showToast('上传失败: ' + (d.error || ''), 'error');
            }
        })
        .catch(err => showToast('请求失败: ' + err, 'error'));
}

let _runBOFId = '';

function runBOF(id) {
    _runBOFId = id;
    fetch('/api/agents')
        .then(r => r.json())
        .then(d => {
            const sel = document.getElementById('run-agent-id');
            sel.innerHTML = '<option value="">-- 选择 Implant --</option>';
            if (d.agents) {
                d.agents.forEach(a => {
                    const opt = document.createElement('option');
                    opt.value = a.id;
                    opt.textContent = a.hostname + ' (' + a.ip + ')  [' + a.id.substring(0,8) + ']';
                    sel.appendChild(opt);
                });
            }
            document.getElementById('run-bof-id').value = id;
            document.getElementById('run-modal').classList.remove('hidden');
        })
        .catch(err => showToast('加载 Implant 列表失败', 'error'));
}

function runBOFSubmit(e) {
    e.preventDefault();
    const bofId = document.getElementById('run-bof-id').value;
    const agentId = document.getElementById('run-agent-id').value;
    const args = document.querySelector('#run-form input[name="args"]').value;
    if (!agentId) { showToast('请选择 Implant', 'error'); return; }

    const fd = new FormData();
    fd.append('agent_id', agentId);
    fd.append('args', args);

    fetch('/api/bof/' + bofId + '/run', { method: 'POST', body: fd })
        .then(r => r.json())
        .then(d => {
            if (d.success) {
                showToast('BOF 已下发执行', 'success');
                closeModal('run-modal');
                setTimeout(refreshResults, 1000);
            } else {
                showToast('执行失败: ' + (d.error || ''), 'error');
            }
        })
        .catch(err => showToast('请求失败: ' + err, 'error'));
}

function downloadBOF(id) {
    window.location.href = '/api/bof/' + id + '/download';
}

function deleteBOF(id) {
    if (!confirm('确认删除此 BOF 文件?')) return;
    fetch('/api/bof/' + id, { method: 'DELETE' })
        .then(r => r.json())
        .then(d => {
            if (d.success) {
                showToast('BOF 已删除', 'success');
                setTimeout(() => location.reload(), 800);
            } else {
                showToast('删除失败: ' + (d.error || ''), 'error');
            }
        })
        .catch(err => showToast('请求失败: ' + err, 'error'));
}

function editBOF(id) {
    fetch('/api/bof/list')
        .then(r => r.json())
        .then(d => {
            if (d.bofs) {
                const bof = d.bofs.find(b => b.id == id);
                if (bof) {
                    document.getElementById('edit-bof-id').value = id;
                    document.getElementById('edit-name').value = bof.name;
                    document.getElementById('edit-description').value = bof.description || '';
                    document.getElementById('edit-modal').classList.remove('hidden');
                }
            }
        });
}

function editBOFSubmit(e) {
    e.preventDefault();
    const id = document.getElementById('edit-bof-id').value;
    const name = document.getElementById('edit-name').value;
    const desc = document.getElementById('edit-description').value;
    const fd = new FormData();
    fd.append('name', name);
    fd.append('description', desc);
    fetch('/api/bof/' + id + '/edit', { method: 'POST', body: fd })
        .then(r => r.json())
        .then(d => {
            if (d.success) {
                showToast('BOF 已更新', 'success');
                closeModal('edit-modal');
                setTimeout(() => location.reload(), 800);
            } else {
                showToast('更新失败: ' + (d.error || ''), 'error');
            }
        })
        .catch(err => showToast('请求失败: ' + err, 'error'));
}

function refreshResults() {
    const el = document.getElementById('recent-results');
    el.innerHTML = '<div class="text-center py-8 text-slate-400"><i class="fa-solid fa-spinner fa-spin text-lg"></i></div>';
    fetch('/api/bof/results?limit=20')
        .then(r => r.json())
        .then(d => {
            if (!d.results || d.results.length === 0) {
                el.innerHTML = '<div class="text-center py-12 text-slate-400"><p class="text-xs">暂无执行记录</p></div>';
                document.getElementById('exec-count').textContent = '0';
                document.getElementById('success-rate').textContent = '-';
                return;
            }
            const total = d.results.length;
            const success = d.results.filter(r => r.status === 'completed').length;
            document.getElementById('exec-count').textContent = total;
            document.getElementById('success-rate').textContent = total > 0 ? Math.round(success/total*100) + '%' : '-';

            el.innerHTML = '<div class="divide-y divide-slate-100">' + d.results.map(r => {
                const statusIcon = r.status === 'completed' ? '<i class="fa-solid fa-check-circle text-emerald-500"></i>' :
                    r.status === 'failed' ? '<i class="fa-solid fa-xmark-circle text-red-500"></i>' :
                    '<i class="fa-solid fa-spinner text-amber-500"></i>';
                const resultText = r.result ? r.result.substring(0, 80) : '';
                return '<div class="px-4 py-2.5 hover:bg-slate-50">' +
                    '<div class="flex items-center justify-between">' +
                    '<div class="flex items-center gap-x-2 min-w-0">' +
                    statusIcon +
                    '<span class="text-xs font-mono text-slate-500">' + (r.agent_name || r.agent_id.substring(0,8)) + '</span>' +
                    '<span class="text-xs text-slate-700 truncate">' + escapeHtml(r.args || '') + '</span>' +
                    '</div>' +
                    '<span class="text-[10px] text-slate-400 shrink-0">' + r.elapsed + '</span>' +
                    '</div>' +
                    (resultText ? '<div class="text-xs text-slate-500 mt-1 ml-6 truncate">' + escapeHtml(resultText) + '</div>' : '') +
                    '</div>';
            }).join('') + '</div>';
        })
        .catch(() => {
            el.innerHTML = '<div class="text-center py-8 text-red-400"><p class="text-xs">加载失败</p></div>';
        });
}

document.addEventListener('DOMContentLoaded', refreshResults);
