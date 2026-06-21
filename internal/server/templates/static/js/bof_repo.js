function loadBOFRepos() {
    fetch('/api/bof/repos').then(r=>r.json()).then(d=>{
        const el = document.getElementById('repo-list');
        if (!d.success || !d.data || d.data.length === 0) {
            el.innerHTML = '<div class="text-center text-xs text-slate-400 py-8">暂无可用 BOF 源</div>';
            return;
        }
        el.innerHTML = d.data.map(r => `
            <div class="flex items-center justify-between p-4 bg-slate-50 rounded-xl">
                <div>
                    <div class="text-sm font-medium">${escapeHtml(r.name)}</div>
                    <div class="text-xs text-slate-500">${escapeHtml(r.description)}</div>
                    <div class="text-[10px] text-slate-400 font-mono mt-0.5">${escapeHtml(r.url)}</div>
                </div>
                <button onclick="window.open('${r.url}','_blank')" class="px-3 py-1.5 bg-purple-600 hover:bg-purple-700 text-white rounded-xl text-xs font-medium">查看</button>
            </div>
        `).join('');
    }).catch(() => document.getElementById('repo-list').innerHTML = '<div class="text-center text-xs text-red-400 py-8">加载失败</div>');
}

function importBOFFromURL() {
    const url = document.getElementById('bof-url').value;
    const name = document.getElementById('bof-name').value;
    if (!url || !name) return showToast('请填写 URL 和名称', 'error');
    const btn = event.target;
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> 导入中';
    fetch('/api/bof/repos/import', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({url, filename: name})})
    .then(r=>r.json()).then(d=>{
        if (d.success) {
            showToast('BOF 导入成功', 'success');
            document.getElementById('bof-import-result').textContent = d.message || '';
            document.getElementById('bof-import-result').classList.remove('hidden');
        } else {
            showToast('导入失败: ' + (d.error || ''), 'error');
        }
    }).catch(err => showToast('请求失败', 'error'))
    .finally(() => { btn.disabled = false; btn.innerHTML = '导入'; });
}

document.addEventListener('DOMContentLoaded', loadBOFRepos);
