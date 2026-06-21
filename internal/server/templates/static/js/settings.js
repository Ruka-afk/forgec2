document.addEventListener('DOMContentLoaded', function() {
    // IntersectionObserver for nav highlighting
    const observer = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                const id = entry.target.id.replace('section-', '');
                document.querySelectorAll('.settings-nav').forEach(a => {
                    a.classList.remove('bg-indigo-50', 'text-indigo-700', 'font-semibold');
                });
                const active = document.querySelector(`.settings-nav[data-section="${id}"]`);
                if (active) active.classList.add('bg-indigo-50', 'text-indigo-700', 'font-semibold');
            }
        });
    }, { rootMargin: '-80px 0px -60% 0px' });
    document.querySelectorAll('section[id^="section-"]').forEach(s => observer.observe(s));

    // Check for updates (version + hot-update)
    fetch('/api/update-check/version').then(r => r.json()).then(data => {
        const badge = document.getElementById('update-status');
        if (badge) {
            if (data.latest_version && !data.is_latest) {
                badge.innerHTML = `<span class="inline-flex items-center gap-x-1 text-xs bg-sky-100 text-sky-700 px-2 py-0.5 rounded-full"><i class="fa-solid fa-circle-up"></i> ${data.latest_version}</span>`;
            } else if (data.check_error) {
                badge.innerHTML = `<span class="text-xs text-slate-400">检查失败</span>`;
            } else {
                badge.innerHTML = `<span class="text-xs text-emerald-600">✓ 最新 (${data.latest_version || 'current'})</span>`;
            }
        }
        const hotBtn = document.getElementById('hot-update-btn');
        if (hotBtn && data.latest_version && !data.is_latest) {
            hotBtn.classList.remove('hidden');
            hotBtn.querySelector('.update-version').textContent = data.latest_version;
        }
    }).catch(() => showToast('更新检查失败', 'error'));

    // Smooth scroll for nav links
    document.querySelectorAll('.settings-nav').forEach(a => {
        a.addEventListener('click', function(e) {
            e.preventDefault();
            const target = document.querySelector(this.getAttribute('href'));
            if (target) target.scrollIntoView({ behavior: 'smooth', block: 'start' });
        });
    });

    // HTMX toast handler: auto-show toast from JSON responses
    document.body.addEventListener('htmx:beforeSwap', function(evt) {
        const xhr = evt.detail.xhr;
        if (xhr) {
            try {
                const ct = xhr.getResponseHeader('Content-Type') || '';
                if (ct.includes('application/json')) {
                    const data = JSON.parse(xhr.responseText);
                    if (data.success && data.message) {
                        setTimeout(function() {
                            showToast(data.message, 'success');
                        }, 100);
                    } else if (data.error) {
                        setTimeout(function() {
                            showToast(data.error, 'error');
                        }, 100);
                    }
                }
            } catch(e) {}
        }
    });
});

function purgeOld(target) {
    const el = document.getElementById('purge-' + target + '-days');
    const days = el ? el.value : 30;
    if (!confirm('清理超过 ' + days + ' 天的' + (target === 'tasks' ? '任务' : '审计日志') + '？')) return;
    const btn = document.getElementById('purge-' + target + '-btn');
    if (btn) { btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i>清理中'; }
    const fd = new FormData(); fd.append('days', days);
    fetch('/settings/purge/' + target, { method: 'POST', body: fd })
        .then(r => r.json()).then(d => {
            if (d.success) { showToast(d.message || '清理完成'); setTimeout(() => location.reload(), 1000); }
            else showToast('失败: ' + (d.error || ''), 'error');
        }).catch(e => showToast('请求失败: ' + e, 'error'))
        .finally(() => { if (btn) { btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-trash mr-1"></i>清理'; } });
}

function vacuumDB() {
    if (!confirm('确认执行数据库 VACUUM？')) return;
    const btn = document.getElementById('vacuum-btn');
    if (btn) { btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i>VACUUM'; }
    fetch('/settings/db/vacuum', { method: 'POST' })
        .then(r => r.json()).then(d => {
            if (d.success) { showToast('VACUUM 完成'); setTimeout(() => location.reload(), 1000); }
            else showToast('失败: ' + (d.error || ''), 'error');
        }).catch(e => showToast('错误: ' + e, 'error'))
        .finally(() => { if (btn) { btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-compress mr-1"></i>VACUUM'; } });
}

function regenerateJWT() {
    if (!confirm('重新生成 JWT 密钥将使所有会话失效，确定？')) return;
    const btn = document.getElementById('jwt-btn');
    if (btn) { btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i>生成中'; }
    fetch('/settings/jwt/regenerate', { method: 'POST' })
        .then(r => r.json()).then(d => {
            if (d.success) { showToast('密钥已重新生成，请重新登录'); setTimeout(() => window.location = '/login', 2000); }
            else showToast('失败: ' + (d.error || ''), 'error');
        }).catch(e => showToast('错误: ' + e, 'error'))
        .finally(() => { if (btn) { btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-rotate mr-1"></i>重新生成'; } });
}

function backupDB() {
    if (!confirm('创建当前数据库的备份？')) return;
    const btn = document.getElementById('backup-btn');
    if (btn) { btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i>备份中'; }
    fetch('/settings/db/backup', { method: 'POST' })
        .then(r => r.json()).then(d => {
            if (d.success) { showToast(d.message || '备份完成'); }
            else showToast('失败: ' + (d.error || ''), 'error');
        }).catch(e => showToast('错误: ' + e, 'error'))
        .finally(() => { if (btn) { btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-copy mr-1"></i>立即备份'; } });
}

function checkForUpdate() {
    const btn = event && event.target || document.querySelector('button[onclick="checkForUpdate()"]');
    const orig = btn.innerHTML;
    btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i>检查中...';
    fetch('/api/update-check/refresh', { method: 'POST' })
        .then(r => r.json()).then(data => {
            const badge = document.getElementById('update-status');
            const hotBtn = document.getElementById('hot-update-btn');
            if (data.update_available) {
                badge.innerHTML = `<span class="inline-flex items-center gap-x-1 text-xs bg-sky-100 text-sky-700 px-2 py-0.5 rounded-full"><i class="fa-solid fa-circle-up"></i> ${data.latest_version}</span>`;
                if (hotBtn) {
                    hotBtn.classList.remove('hidden');
                    hotBtn.querySelector('.update-version').textContent = data.latest_version;
                }
                showToast('发现新版本 ' + data.latest_version, 'info');
            } else if (data.check_error) {
                badge.innerHTML = `<span class="text-xs text-red-500">检查失败</span>`;
                showToast('检查更新失败: ' + data.check_error, 'error');
            } else {
                badge.innerHTML = `<span class="text-xs text-emerald-600">✓ 已是最新</span>`;
                if (hotBtn) hotBtn.classList.add('hidden');
                showToast('已是最新版本', 'success');
            }
        }).catch(e => {
            showToast('检查更新请求失败', 'error');
        }).finally(() => {
            btn.disabled = false; btn.innerHTML = orig;
        });
}

function hotUpdate() {
    if (!confirm('确认热更新到最新版本？服务器将自动下载二进制文件并重启。')) return;
    const btn = document.querySelector('#hot-update-btn button');
    if (btn) { btn.disabled = true; btn.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i>更新中...'; }
    fetch('/api/update-check/hot-update', { method: 'POST' })
        .then(r => r.json()).then(d => {
            if (d.success) {
                showToast(d.message || '正在更新...', 'success');
                setTimeout(function() {
                    document.body.innerHTML = '<div style="display:flex;align-items:center;justify-content:center;height:100vh;font-family:sans-serif"><div style="text-align:center"><h2 style="margin-bottom:8px">服务器正在更新...</h2><p style="color:#666">自动重启后请刷新页面</p></div></div>';
                }, 2000);
            } else {
                showToast('更新失败: ' + (d.error || ''), 'error');
                if (btn) { btn.disabled = false; btn.innerHTML = '<i class="fa-solid fa-cloud-arrow-down mr-1"></i>重试'; }
            }
        }).catch(e => showToast('请求失败: ' + e, 'error'))
        .finally(() => {});
}
