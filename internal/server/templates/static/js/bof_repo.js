function loadBOFRepos() {
    fetch('/api/bof/repos').then(r=>r.json()).then(d=>{
        const el = document.getElementById('repo-list');
        if (!d.success || !d.data || d.data.length === 0) {
            el.innerHTML = '<div class="text-center text-xs text-slate-400 py-8">' + __t('No available BOF sources') + '</div>';
            return;
        }
        el.innerHTML = d.data.map(r => `
            <div class="flex items-center justify-between p-4 bg-slate-50 rounded-xl">
                <div>
                    <div class="text-sm font-medium">${escapeHtml(r.name)}</div>
                    <div class="text-xs text-slate-500">${escapeHtml(r.description)}</div>
                    <div class="text-[10px] text-slate-400 font-mono mt-0.5">${escapeHtml(r.url)}</div>
                </div>
                <button onclick="window.open('${r.url}','_blank')" class="px-3 py-1.5 bg-purple-600 hover:bg-purple-700 text-white rounded-xl text-xs font-medium">${__t('View')}</button>
            </div>
        `).join('');
    }).catch(() => document.getElementById('repo-list').innerHTML = '<div class="text-center text-xs text-red-400 py-8">' + __t('Load failed') + '</div>');
}

function importBOFFromURL() {
    const url = document.getElementById('bof-url').value;
    const name = document.getElementById('bof-name').value;
    if (!url || !name) return showToast(__t('Please fill in URL and name'), 'error');
    const btn = event.target;
    btn.disabled = true;
    btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin"></i> ' + __t('Importing');
    fetch('/api/bof/repos/import', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({url, filename: name})})
    .then(r=>r.json()).then(d=>{
        if (d.success) {
            showToast(__t('BOF import successful'), 'success');
            document.getElementById('bof-import-result').textContent = d.message || '';
            document.getElementById('bof-import-result').classList.remove('hidden');
        } else {
            showToast(__tf('Import failed: {0}', d.error || ''), 'error');
        }
    }).catch(err => showToast(__t('Request failed'), 'error'))
    .finally(() => { btn.disabled = false; btn.innerHTML = __t('Import'); });
}

document.addEventListener('DOMContentLoaded', loadBOFRepos);
