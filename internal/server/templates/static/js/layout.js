function showToast(message, type = 'success') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    const colorMap = {
        'success': 'bg-emerald-100 border-emerald-300 text-emerald-700',
        'error': 'bg-red-100 border-red-300 text-red-700',
        'warning': 'bg-amber-100 border-amber-300 text-amber-700',
        'info': 'bg-sky-100 border-sky-300 text-sky-700',
    };
    const iconMap = {
        'success': 'fa-check-circle',
        'error': 'fa-exclamation-circle',
        'warning': 'fa-triangle-exclamation',
        'info': 'fa-circle-info',
    };
    toast.className = `px-4 py-3 rounded-2xl shadow-xl flex items-center gap-x-3 text-sm border ${colorMap[type] || colorMap.success}`;
    toast.innerHTML = `<i class="fa-solid ${iconMap[type] || iconMap.success}"></i>`;
    const msgSpan = document.createElement('span');
    msgSpan.textContent = message;
    toast.appendChild(msgSpan);
    container.appendChild(toast);
    setTimeout(() => {
        toast.style.transition = 'all 0.3s ease';
        toast.style.opacity = '0';
        setTimeout(() => toast.remove(), 200);
    }, 2800);
}

// Track known online users to avoid spamming toasts on page changes
let knownOnlineUsers = new Set();

function updateOnlineUsers(users) {
    const container = document.getElementById('online-users');
    const count = document.getElementById('online-count');
    if (!container) return;
    if (count) count.textContent = (users && users.length) || 0;
    if (!users || users.length === 0) {
        container.innerHTML = '<div class="text-[11px] text-slate-400 italic">无其他在线用户</div>';
        return;
    }
    const currentUser = window.currentUserDisplayName || '';
    container.innerHTML = users.map(u => {
        const isMe = u.username === currentUser;
        const roleIcon = u.role === 'admin' ? 'fa-crown text-amber-500' :
                       u.role === 'viewer' ? 'fa-eye text-slate-400' :
                       'fa-user text-sky-500';
        const badge = u.role === 'admin' ? '<span class="text-[8px] bg-indigo-100 text-indigo-700 px-1 rounded">ADMIN</span>' :
                     u.role === 'viewer' ? '<span class="text-[8px] bg-slate-100 text-slate-500 px-1 rounded">VIEW</span>' :
                     '<span class="text-[8px] bg-sky-100 text-sky-600 px-1 rounded">OP</span>';
        const nameClass = isMe ? 'text-indigo-600 font-semibold' : 'text-slate-700';
        const selfLabel = isMe ? '<span class="text-[8px] text-indigo-400 ml-0.5">(你)</span>' : '';
        const pageInfo = u.current_page && !isMe
            ? `<span class="text-[9px] text-slate-400 block truncate pl-4">${escapeHtml(u.current_page)}</span>`
            : '';
        return `<div class="flex items-center gap-x-1.5 text-[11px]"><span class="w-1.5 h-1.5 bg-emerald-500 rounded-full shrink-0"></span><i class="fa-solid ${roleIcon} text-[10px] w-3 text-center shrink-0"></i><span class="${nameClass} truncate max-w-[80px]">${u.username}${selfLabel}</span>${badge}${pageInfo}</div>`;
    }).join('');

    // Detect newly online users for toast
    if (knownOnlineUsers.size > 0) {
        const newNames = users.map(u => u.username);
        newNames.forEach(name => {
            if (!knownOnlineUsers.has(name) && name !== currentUser) {
                showToast(name + ' 上线', 'info');
            }
        });
    }
    knownOnlineUsers = new Set(users.map(u => u.username));
}

function toggleChat() {
    const w = document.getElementById('chat-window');
    w.classList.toggle('hidden');
    if (!w.classList.contains('hidden')) {
        document.getElementById('chat-badge').classList.add('hidden');
        if (document.getElementById('chat-messages').children.length <= 1) {
            loadChatHistory();
        }
    }
}

function loadChatHistory() {
    fetch('/api/collab/chat').then(r => r.json()).then(d => {
        if (d.success && d.messages) {
            const el = document.getElementById('chat-messages');
            el.innerHTML = '';
            d.messages.forEach(m => appendChatMessage(m, m.role || ''));
            el.scrollTop = el.scrollHeight;
        }
    }).catch(() => showToast('聊天消息加载失败', 'error'));
}

function appendChatMessage(msg, role) {
    const el = document.getElementById('chat-messages');
    if (!el) return;
    const chatWin = document.getElementById('chat-window');
    if (chatWin && chatWin.classList.contains('hidden') && msg.username !== '[系统]' && msg.username !== window.currentUserDisplayName) {
        const badge = document.getElementById('chat-badge');
        if (badge) badge.classList.remove('hidden');
    }
    if (el.children.length === 1 && el.children[0].textContent.includes('连接')) {
        el.innerHTML = '';
    }
    const time = msg.created_at ? new Date(msg.created_at).toLocaleTimeString() : '';
    const div = document.createElement('div');
    const isSystem = msg.username === '[系统]';
    if (isSystem) {
        div.className = 'bg-slate-100 rounded-xl px-3 py-2 border border-slate-200';
        div.innerHTML = `<div class="flex items-center gap-2 text-[10px] text-slate-500"><i class="fa-solid fa-circle-info text-slate-400"></i><span>${escapeHtml(msg.content)}</span><span class="ml-auto text-[9px] text-slate-400">${time}</span></div>`;
    } else {
        const roleColor = role === 'admin' ? 'text-amber-600' : role === 'viewer' ? 'text-slate-500' : 'text-indigo-600';
        div.className = 'bg-white rounded-xl px-3 py-2 border border-slate-100';
        div.innerHTML = `<div class="flex items-center justify-between"><span class="text-xs font-semibold ${roleColor}">${escapeHtml(msg.username)}</span><span class="text-[9px] text-slate-400">${time}</span></div><div class="text-xs text-slate-700 mt-0.5">${escapeHtml(msg.content)}</div>`;
    }
    el.appendChild(div);
    el.scrollTop = el.scrollHeight;
}

function sendChat(e) {
    e.preventDefault();
    const input = document.getElementById('chat-input');
    const content = input.value.trim();
    if (!content) return;
    input.value = '';
    const fd = new FormData();
    fd.append('content', content);
    fetch('/api/collab/chat', { method: 'POST', headers: { 'X-CSRF-Token': getCSRFToken() }, body: fd }).catch(() => showToast('聊天发送失败', 'error'));
}

function updateAgentLockUI(agentId, username) {
    const indicator = document.getElementById('lock-indicator-' + agentId);
    if (!indicator) return;
    if (username) {
        indicator.innerHTML = `<span class="text-xs text-amber-600"><i class="fa-solid fa-lock mr-1"></i>${username}</span>`;
    } else {
        indicator.innerHTML = '';
    }
}

function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

var _ws = null;

function wsSend(data) {
    if (_ws && _ws.readyState === WebSocket.OPEN) {
        _ws.send(JSON.stringify(data));
    }
}

function sendWSPageUpdate() {
    const path = window.location.pathname;
    wsSend({ type: 'page_update', page: path });
    const m = path.match(/^\/agents\/([a-fA-F0-9-]+)/);
    if (m) {
        wsSend({ type: 'agent_view', agent_id: m[1] });
    }
}

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    _ws = new WebSocket(protocol + '//' + window.location.host + '/ws');

    _ws.onopen = function() {
        sendWSPageUpdate();
    };

    _ws.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            if (data.type === 'agent_online') {
                showToast(`新 Implant 上线: ${data.hostname} (${data.ip})`, 'success');
                if (window.location.pathname === '/agents' || window.location.pathname === '/dashboard') {
                    setTimeout(() => window.location.reload(), 1000);
                }
            } else if (data.type === 'task_update') {
                const agentId = data.agent_id || '';
                const taskType = data.task_type || '';
                const status = data.status || '';
                const cmd = (data.command || '').substring(0, 50);

                if (taskType === 'ls') return;

                if (status === 'completed') {
                    showToast(`任务完成 [${taskType}]: ${cmd}`, 'success');
                    const path = window.location.pathname;
                    if (path === '/tasks' || path.includes('/agents/' + agentId)) {
                        setTimeout(() => {
                            if (path === '/tasks') {
                                location.reload();
                            } else if (typeof refreshTasks === 'function') {
                                refreshTasks(agentId);
                            }
                        }, 800);
                    }
                } else if (status === 'failed') {
                    showToast(`任务失败 [${taskType}]: ${cmd}`, 'error');
                }
            } else if (data.type === 'update_available') {
                showToast(`新版本可用: ${data.latest} (当前: ${data.current})`, 'info');
                const banner = document.getElementById('update-banner');
                if (banner) {
                    banner.classList.remove('hidden');
                    banner.querySelector('.update-version').textContent = data.latest;
                    banner.querySelector('.update-download').href = data.download_url || '#';
                }
            } else if (data.type === 'user_online' || data.type === 'user_offline') {
                updateOnlineUsers(data.users);
                if (data.type === 'user_offline' && data.username && data.username !== window.currentUserDisplayName) {
                    showToast(data.username + ' 下线', 'warning');
                }
            } else if (data.type === 'chat') {
                appendChatMessage(data.message, data.message.role);
            } else if (data.type === 'agent_locked') {
                showToast(`Implant ${data.agent_id.substring(0,8)} 已被 ${data.username} 锁定`, 'info');
                updateAgentLockUI(data.agent_id, data.username);
                const lockEl = document.querySelector('#lock-col-' + data.agent_id + ' .lock-text');
                if (lockEl) lockEl.textContent = '🔒 ' + data.username;
            } else if (data.type === 'agent_unlocked') {
                showToast(`Implant ${data.agent_id.substring(0,8)} 已解锁`, 'info');
                updateAgentLockUI(data.agent_id, null);
                const lockEl = document.querySelector('#lock-col-' + data.agent_id + ' .lock-text');
                if (lockEl) lockEl.textContent = '';
            } else if (data.type === 'user_viewing_agent') {
                window.dispatchEvent(new CustomEvent('collab:viewing_agent', { detail: data }));
            }
        } catch (e) {
            console.error('Failed to parse WebSocket message:', e);
        }
    };

    _ws.onerror = function(error) {
        console.error('WebSocket error:', error);
    };

    _ws.onclose = function() {
        _ws = null;
        setTimeout(connectWebSocket, 5000);
    };
}

document.addEventListener('DOMContentLoaded', function() {
    connectWebSocket();

    const csrfToken = getCookie('csrf_token');
    if (csrfToken && typeof htmx !== 'undefined' && htmx.config && htmx.config.headers) {
        htmx.config.headers['X-CSRF-Token'] = csrfToken;
    }
});

document.addEventListener('htmx:afterSettle', function() {
    sendWSPageUpdate();
});

window.addEventListener('popstate', function() {
    setTimeout(sendWSPageUpdate, 100);
});

function getCookie(name) {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return parts.pop().split(';').shift();
    return '';
}

window.getCSRFToken = function() {
    return window.csrfToken || getCookie('csrf_token');
};

(function patchFetchForCSRF() {
    const originalFetch = window.fetch;
    window.fetch = function (resource, init) {
        init = init || {};
        const method = (init.method || 'GET').toUpperCase();
        if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
            let headers = init.headers || {};
            const hasCSRF = headers && (headers['X-CSRF-Token'] || headers['x-csrf-token'] ||
                            (headers instanceof Headers && headers.has('X-CSRF-Token')));
            if (!hasCSRF) {
                const token = window.getCSRFToken ? window.getCSRFToken() : '';
                if (token) {
                    if (headers instanceof Headers) {
                        headers.set('X-CSRF-Token', token);
                    } else {
                        headers = headers || {};
                        headers['X-CSRF-Token'] = token;
                    }
                    init.headers = headers;
                }
            }
        }
        return originalFetch(resource, init);
    };
})();

window.toggleNavSection = function(headerEl) {
    headerEl.classList.toggle('collapsed');
    const section = headerEl.nextElementSibling;
    const key = 'forgec2_nav_collapsed';
    let collapsed = JSON.parse(localStorage.getItem(key) || '{}');
    const label = headerEl.textContent.trim();
    collapsed[label] = headerEl.classList.contains('collapsed');
    localStorage.setItem(key, JSON.stringify(collapsed));
};

(function restoreNavState() {
    try {
        const key = 'forgec2_nav_collapsed';
        const collapsed = JSON.parse(localStorage.getItem(key) || '{}');
        document.addEventListener('DOMContentLoaded', function() {
            const headers = document.querySelectorAll('#sidebar-nav .section-header');
            headers.forEach(h => {
                const label = h.textContent.trim();
                if (collapsed[label]) {
                    h.classList.add('collapsed');
                }
            });
        });
    } catch(e){}
})();

window.filterNav = function(query) {
    const q = (query || '').toLowerCase().trim();
    const nav = document.getElementById('sidebar-nav');
    if (!nav) return;

    const links = nav.querySelectorAll('a');
    const sections = nav.querySelectorAll('.section-header');

    links.forEach(link => {
        const text = link.textContent.toLowerCase();
        const match = !q || text.includes(q);
        link.style.display = match ? '' : 'none';
    });

    sections.forEach(header => {
        const next = header.nextElementSibling;
        if (!next || !next.classList.contains('nav-section')) return;
        const visible = Array.from(next.querySelectorAll('a')).some(a => a.style.display !== 'none');
        header.style.display = (visible || !q) ? '' : 'none';
        next.style.display = (visible || !q) ? '' : 'none';
    });
};

window.showCreateListenerModal = function() {
    window.location.href = '/listeners';
};

var _modalAgentId = '';
var _modalTaskId = 0;

function toggleMobileSidebar() {
    const sidebar = document.getElementById('mobile-sidebar');
    const overlay = document.getElementById('mobile-overlay');
    sidebar.classList.toggle('open');
    overlay.classList.toggle('active');

    const mobileNav = document.getElementById('mobile-sidebar-nav');
    if (mobileNav.children.length === 0) {
        const desktopNav = document.getElementById('sidebar-nav');
        if (desktopNav) {
            mobileNav.innerHTML = desktopNav.innerHTML;
        }
    }
}

document.addEventListener('click', function(e) {
    if (e.target.closest('#mobile-sidebar-nav a')) {
        toggleMobileSidebar();
    }
});

function showRecentTaskModal(el) {
    const taskId = el.getAttribute('data-task-id');
    const agentId = el.getAttribute('data-agent-id');
    if (!taskId) return;
    _modalTaskId = taskId;
    _modalAgentId = agentId || '';

    const resultBox = document.getElementById('modal-result-box');
    const errorBox = document.getElementById('modal-error-box');
    resultBox.classList.add('hidden');
    errorBox.classList.add('hidden');

    const hostnameEl = el.querySelector('.font-medium.text-slate-900');
    document.getElementById('modal-agent').textContent = hostnameEl ? hostnameEl.textContent : '-';
    const cmdEl = el.querySelector('.font-mono.truncate');
    document.getElementById('modal-command').textContent = cmdEl ? cmdEl.textContent : '-';
    const timeEl = el.querySelector('.font-mono.text-slate-400');
    document.getElementById('modal-time').textContent = timeEl ? timeEl.textContent : '';

    document.getElementById('task-modal').classList.remove('hidden');
    document.getElementById('task-modal').classList.add('flex');

    fetch(`/tasks/${taskId}`)
        .then(r => r.json())
        .then(data => {
            if (data.error) {
                document.getElementById('modal-command').textContent = '(详情加载失败)';
                return;
            }
            document.getElementById('modal-type').textContent = data.type || '-';
            document.getElementById('modal-agent').textContent = data.agent || document.getElementById('modal-agent').textContent;
            document.getElementById('modal-time').textContent = data.created || document.getElementById('modal-time').textContent;
            document.getElementById('modal-command').textContent = data.command || '-';

            if (data.result) {
                document.getElementById('modal-result').textContent = data.result;
                resultBox.classList.remove('hidden');
            }
            if (data.error) {
                document.getElementById('modal-error').textContent = data.error;
                errorBox.classList.remove('hidden');
            }
            document.getElementById('modal-status').innerHTML = data.status === 'completed' ?
                '<span class="px-2 py-0.5 bg-emerald-100 text-emerald-700 rounded text-xs">已完成</span>' :
                (data.status === 'failed' ? '<span class="px-2 py-0.5 bg-red-100 text-red-700 rounded text-xs">失败</span>' : '<span class="px-2 py-0.5 bg-amber-100 text-amber-700 rounded text-xs">' + (data.status || '') + '</span>');
        })
        .catch(err => {
            console.error('fetch task detail error', err);
        });
}

function hideTaskModal() {
    const modal = document.getElementById('task-modal');
    modal.classList.remove('flex');
    modal.classList.add('hidden');
}

function rerunFromModal() {
    if (!_modalAgentId || !_modalTaskId) return;
    fetch(`/agents/${_modalAgentId}/task/${_modalTaskId}/rerun`, { method: 'POST' })
        .then(r => r.json())
        .then(data => {
            if (data.success) {
                showToast('任务已重新创建', 'success');
                hideTaskModal();
                setTimeout(() => location.reload(), 1200);
            } else {
                showToast('重跑失败: ' + (data.error || ''), 'error');
            }
        })
        .catch(err => showToast('请求失败: ' + err, 'error'));
}
