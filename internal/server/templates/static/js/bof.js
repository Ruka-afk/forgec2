function showUploadModal() { document.getElementById('upload-modal').classList.remove('hidden'); }
function closeModal(id) { document.getElementById(id).classList.add('hidden'); }

function uploadBOF(e) {
    e.preventDefault();
    const form = document.getElementById('upload-form');
    const fd = new FormData(form);
    apiFetch('/api/bof/upload', { method: 'POST', body: fd })
        .then(d => {
            if (d.success) { showToast(__('BOF uploaded'), 'success'); closeModal('upload-modal'); setTimeout(() => location.reload(), 800); }
            else { showToast(__tf('Upload failed: {0}', d.error || ''), 'error'); }
        }).catch(err => showToast(__tf('Request failed: {0}', err), 'error'));
}

let _runBOFId = '';

function runBOF(id) {
    _runBOFId = id;
    apiFetch('/api/agents')
        .then(d => {
            const sel = document.getElementById('run-agent-id');
            sel.innerHTML = '<option value="">' + __('-- Select agent --') + '</option>';
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
        }).catch(err => showToast(__tf('Failed to load agent list', err), 'error'));
}

function runBOFSubmit(e) {
    e.preventDefault();
    const bofId = document.getElementById('run-bof-id').value;
    const agentId = document.getElementById('run-agent-id').value;
    const args = document.querySelector('#run-form input[name="args"]').value;
    if (!agentId) { showToast(__('Please select an agent'), 'error'); return; }
    const fd = new FormData();
    fd.append('agent_id', agentId);
    fd.append('args', args);
    apiFetch('/api/bof/' + bofId + '/run', { method: 'POST', body: fd })
        .then(d => {
            if (d.success) { showToast(__('BOF execution sent'), 'success'); closeModal('run-modal'); setTimeout(refreshResults, 1000); }
            else { showToast(__tf('Execution failed: {0}', d.error || ''), 'error'); }
        }).catch(err => showToast(__tf('Request failed: {0}', err), 'error'));
}

function downloadBOF(id) { window.location.href = '/api/bof/' + id + '/download'; }

function deleteBOF(id) {
    if (!confirm(__('Delete this BOF file?'))) return;
    apiFetch('/api/bof/' + id, { method: 'DELETE' })
        .then(d => {
            if (d.success) { showToast(__('BOF deleted'), 'success'); setTimeout(() => location.reload(), 800); }
            else { showToast(__tf('Delete failed: {0}', d.error || ''), 'error'); }
        }).catch(err => showToast(__tf('Request failed: {0}', err), 'error'));
}

function editBOF(id) {
    apiFetch('/api/bof/list')
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
        }).catch(() => {});
}

function editBOFSubmit(e) {
    e.preventDefault();
    const id = document.getElementById('edit-bof-id').value;
    const name = document.getElementById('edit-name').value;
    const desc = document.getElementById('edit-description').value;
    const fd = new FormData();
    fd.append('name', name);
    fd.append('description', desc);
    apiFetch('/api/bof/' + id + '/edit', { method: 'POST', body: fd })
        .then(d => {
            if (d.success) { showToast(__('BOF updated'), 'success'); closeModal('edit-modal'); setTimeout(() => location.reload(), 800); }
            else { showToast(__tf('Update failed: {0}', d.error || ''), 'error'); }
        }).catch(err => showToast(__tf('Request failed: {0}', err), 'error'));
}

function refreshResults() {
    const el = document.getElementById('recent-results');
    if (!el) return;
    el.innerHTML = '<div class="text-center py-8 text-slate-400"><i class="fa-solid fa-spinner fa-spin text-lg"></i></div>';
    apiFetch('/api/bof/results?limit=20')
        .then(d => {
            if (!d.results || d.results.length === 0) {
                el.innerHTML = '<div class="text-center py-12 text-slate-400"><p class="text-xs">' + __('No execution records') + '</p></div>';
                document.getElementById('exec-count').textContent = '0';
                document.getElementById('success-rate').textContent = '-';
                return;
            }
            const total = d.results.length;
            const success = d.results.filter(r => r.status === 'completed').length;
            document.getElementById('exec-count').textContent = total;
            document.getElementById('success-rate').textContent = total > 0 ? Math.round(success/total*100) + '%' : '-';
            el.innerHTML = '<div class="divide-y divide-slate-100">' + d.results.map(r => {
                const statusIcon = r.status === 'completed' ? '<i class="fa-solid fa-check-circle text-emerald-500"></i>' : r.status === 'failed' ? '<i class="fa-solid fa-xmark-circle text-red-500"></i>' : '<i class="fa-solid fa-spinner text-amber-500"></i>';
                const resultText = r.result ? r.result.substring(0, 80) : '';
                return '<div class="px-4 py-2.5 hover:bg-slate-50"><div class="flex items-center justify-between"><div class="flex items-center gap-x-2 min-w-0">' + statusIcon + '<span class="text-xs font-mono text-slate-500">' + (r.agent_name || r.agent_id.substring(0,8)) + '</span><span class="text-xs text-slate-700 truncate">' + escapeHtml(r.args || '') + '</span></div><span class="text-[10px] text-slate-400 shrink-0">' + r.elapsed + '</span></div>' + (resultText ? '<div class="text-xs text-slate-500 mt-1 ml-6 truncate">' + escapeHtml(resultText) + '</div>' : '') + '</div>';
            }).join('') + '</div>';
        }).catch(() => { el.innerHTML = '<div class="text-center py-8 text-red-400"><p class="text-xs">' + __('Load failed') + '</p></div>'; });
}

document.addEventListener('DOMContentLoaded', function() {
    if (!document.getElementById('recent-results')) return;
    refreshResults();
});
