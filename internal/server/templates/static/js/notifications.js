const NOTIFICATION_STORAGE_KEY = 'forgec2_notifications';
const NOTIFICATION_SETTINGS_KEY = 'forgec2_notification_settings';
const MAX_NOTIFICATIONS = 200;

const NotificationSources = {
    AGENT_ONLINE: 'agent_online',
    AGENT_OFFLINE: 'agent_offline',
    USER_ONLINE: 'user_online',
    USER_OFFLINE: 'user_offline',
    TASK_COMPLETE: 'task_complete',
    TASK_FAIL: 'task_fail',
    CREDENTIAL_FOUND: 'credential_found',
    SYSTEM_ALERT: 'system_alert'
};

const NotificationTypes = {
    INFO: 'info',
    SUCCESS: 'success',
    WARNING: 'warning',
    ERROR: 'error'
};

const SourceCategories = {
    [NotificationSources.AGENT_ONLINE]: 'agent',
    [NotificationSources.AGENT_OFFLINE]: 'agent',
    [NotificationSources.USER_ONLINE]: 'user',
    [NotificationSources.USER_OFFLINE]: 'user',
    [NotificationSources.TASK_COMPLETE]: 'task',
    [NotificationSources.TASK_FAIL]: 'task',
    [NotificationSources.CREDENTIAL_FOUND]: 'task',
    [NotificationSources.SYSTEM_ALERT]: 'system'
};

const SourceLabels = {
    [NotificationSources.AGENT_ONLINE]: 'Agent 上线',
    [NotificationSources.AGENT_OFFLINE]: 'Agent 下线',
    [NotificationSources.USER_ONLINE]: '用户上线',
    [NotificationSources.USER_OFFLINE]: '用户下线',
    [NotificationSources.TASK_COMPLETE]: '任务完成',
    [NotificationSources.TASK_FAIL]: '任务失败',
    [NotificationSources.CREDENTIAL_FOUND]: '凭据发现',
    [NotificationSources.SYSTEM_ALERT]: '系统告警'
};

const SourceIcons = {
    [NotificationSources.AGENT_ONLINE]: 'fa-plug-circle-check text-emerald-500',
    [NotificationSources.AGENT_OFFLINE]: 'fa-plug-circle-xmark text-red-500',
    [NotificationSources.USER_ONLINE]: 'fa-user-check text-blue-500',
    [NotificationSources.USER_OFFLINE]: 'fa-user-xmark text-slate-400',
    [NotificationSources.TASK_COMPLETE]: 'fa-circle-check text-emerald-500',
    [NotificationSources.TASK_FAIL]: 'fa-circle-xmark text-red-500',
    [NotificationSources.CREDENTIAL_FOUND]: 'fa-key text-amber-500',
    [NotificationSources.SYSTEM_ALERT]: 'fa-triangle-exclamation text-amber-500'
};

const DefaultNotificationSettings = {
    enabled: true,
    sound_enabled: false,
    desktop_enabled: false,
    sources: {
        [NotificationSources.AGENT_ONLINE]: { in_app: true, desktop: true, sound: false },
        [NotificationSources.AGENT_OFFLINE]: { in_app: true, desktop: true, sound: false },
        [NotificationSources.USER_ONLINE]: { in_app: true, desktop: true, sound: false },
        [NotificationSources.USER_OFFLINE]: { in_app: true, desktop: false, sound: false },
        [NotificationSources.TASK_COMPLETE]: { in_app: true, desktop: false, sound: false },
        [NotificationSources.TASK_FAIL]: { in_app: true, desktop: true, sound: true },
        [NotificationSources.CREDENTIAL_FOUND]: { in_app: true, desktop: true, sound: true },
        [NotificationSources.SYSTEM_ALERT]: { in_app: true, desktop: true, sound: true }
    }
};

let _notificationSettings = null;
let _notifications = [];
let _currentFilter = 'all';
let _notificationAudio = null;

const NotificationCenter = {
    init() {
        this.loadSettings();
        this.loadNotifications();
        this.initUI();
        this.updateBadge();
    },

    loadSettings() {
        try {
            const saved = localStorage.getItem(NOTIFICATION_SETTINGS_KEY);
            if (saved) {
                _notificationSettings = JSON.parse(saved);
                _notificationSettings = this.mergeSettings(DefaultNotificationSettings, _notificationSettings);
            } else {
                _notificationSettings = JSON.parse(JSON.stringify(DefaultNotificationSettings));
            }
        } catch (e) {
            _notificationSettings = JSON.parse(JSON.stringify(DefaultNotificationSettings));
        }
    },

    saveSettings() {
        try {
            localStorage.setItem(NOTIFICATION_SETTINGS_KEY, JSON.stringify(_notificationSettings));
        } catch (e) {
            console.error('Failed to save notification settings:', e);
        }
    },

    mergeDefaults(target, defaults) {
        const result = { ...defaults };
        for (const key in target) {
            if (typeof target[key] === 'object' && target[key] !== null && !Array.isArray(target[key])) {
                result[key] = this.mergeDefaults(target[key], defaults[key] || {});
            } else {
                result[key] = target[key];
            }
        }
        return result;
    },

    loadNotifications() {
        try {
            const saved = localStorage.getItem(NOTIFICATION_STORAGE_KEY);
            if (saved) {
                _notifications = JSON.parse(saved);
            }
        } catch (e) {
            _notifications = [];
        }
    },

    saveNotifications() {
        try {
            if (_notifications.length > MAX_NOTIFICATIONS) {
                _notifications = _notifications.slice(0, MAX_NOTIFICATIONS);
            }
            localStorage.setItem(NOTIFICATION_STORAGE_KEY, JSON.stringify(_notifications));
        } catch (e) {
            console.error('Failed to save notifications:', e);
        }
    },

    addNotification(source, title, message, data = {}) {
        if (!_notificationSettings.enabled) return;

        const sourceConfig = _notificationSettings.sources[source];
        if (!sourceConfig || !sourceConfig.in_app) return;

        const notification = {
            id: Date.now() + '_' + Math.random().toString(36).substr(2, 9),
            source: source,
            type: this.getTypeFromSource(source),
            title: title,
            message: message,
            data: data,
            read: false,
            timestamp: new Date().toISOString()
        };

        _notifications.unshift(notification);
        this.saveNotifications();
        this.updateBadge();
        this.renderNotifications();

        if (sourceConfig.desktop && _notificationSettings.desktop_enabled) {
            this.showDesktopNotification(title, message, source);
        }

        if (sourceConfig.sound && _notificationSettings.sound_enabled) {
            this.playSound();
        }

        return notification;
    },

    getTypeFromSource(source) {
        switch (source) {
            case NotificationSources.AGENT_ONLINE:
            case NotificationSources.TASK_COMPLETE:
            case NotificationSources.CREDENTIAL_FOUND:
                return NotificationTypes.SUCCESS;
            case NotificationSources.AGENT_OFFLINE:
            case NotificationSources.TASK_FAIL:
                return NotificationTypes.ERROR;
            case NotificationSources.SYSTEM_ALERT:
                return NotificationTypes.WARNING;
            default:
                return NotificationTypes.INFO;
        }
    },

    markAsRead(id) {
        const notification = _notifications.find(n => n.id === id);
        if (notification) {
            notification.read = true;
            this.saveNotifications();
            this.updateBadge();
            this.renderNotifications();
        }
    },

    markAllAsRead() {
        _notifications.forEach(n => n.read = true);
        this.saveNotifications();
        this.updateBadge();
        this.renderNotifications();
    },

    clearAll() {
        if (!confirm('确定清空所有通知吗？')) return;
        _notifications = [];
        this.saveNotifications();
        this.updateBadge();
        this.renderNotifications();
    },

    deleteNotification(id) {
        _notifications = _notifications.filter(n => n.id !== id);
        this.saveNotifications();
        this.updateBadge();
        this.renderNotifications();
    },

    getUnreadCount() {
        return _notifications.filter(n => !n.read).length;
    },

    getFilteredNotifications() {
        if (_currentFilter === 'all') {
            return _notifications;
        }
        return _notifications.filter(n => SourceCategories[n.source] === _currentFilter);
    },

    setFilter(filter) {
        _currentFilter = filter;
        this.renderNotifications();
        this.updateFilterTabs();
    },

    updateBadge() {
        const badge = document.getElementById('notification-badge');
        if (!badge) return;

        const count = this.getUnreadCount();
        if (count > 0) {
            badge.textContent = count > 99 ? '99+' : count;
            badge.classList.remove('hidden');
        } else {
            badge.classList.add('hidden');
        }
    },

    initUI() {
        const bellBtn = document.getElementById('notification-bell-btn');
        const panel = document.getElementById('notification-panel');

        if (bellBtn && panel) {
            bellBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                this.togglePanel();
            });
        }

        document.addEventListener('click', (e) => {
            if (panel && !panel.classList.contains('hidden') && !panel.contains(e.target) && !bellBtn.contains(e.target)) {
                this.closePanel();
            }
        });

        this.updateFilterTabs();
    },

    togglePanel() {
        const panel = document.getElementById('notification-panel');
        if (!panel) return;

        if (panel.classList.contains('hidden')) {
            panel.classList.remove('hidden');
            this.renderNotifications();
        } else {
            panel.classList.add('hidden');
        }
    },

    closePanel() {
        const panel = document.getElementById('notification-panel');
        if (panel) {
            panel.classList.add('hidden');
        }
    },

    updateFilterTabs() {
        const tabs = document.querySelectorAll('[data-filter]');
        tabs.forEach(tab => {
            const filter = tab.dataset.filter;
            if (filter === _currentFilter) {
                tab.classList.add('bg-indigo-50', 'text-indigo-700', 'dark:bg-indigo-900/30', 'dark:text-indigo-400');
            } else {
                tab.classList.remove('bg-indigo-50', 'text-indigo-700', 'dark:bg-indigo-900/30', 'dark:text-indigo-400');
            }
        });
    },

    renderNotifications() {
        const list = document.getElementById('notification-list');
        if (!list) return;

        const filtered = this.getFilteredNotifications();

        if (filtered.length === 0) {
            list.innerHTML = `
                <div class="py-12 text-center">
                    <div class="text-4xl mb-3 text-slate-300 dark:text-slate-600">
                        <i class="fa-regular fa-bell"></i>
                    </div>
                    <div class="text-sm text-slate-400 dark:text-slate-500">暂无通知</div>
                </div>
            `;
            return;
        }

        list.innerHTML = filtered.map(n => this.renderNotificationItem(n)).join('');
    },

    renderNotificationItem(notification) {
        const iconClass = SourceIcons[notification.source] || 'fa-circle-info text-slate-500';
        const timeAgo = this.formatTimeAgo(notification.timestamp);
        const readClass = notification.read ? 'opacity-60' : '';

        return `
            <div class="notification-item p-3 hover:bg-slate-50 dark:hover:bg-slate-700/50 cursor-pointer transition-colors border-b border-slate-100 dark:border-slate-700 ${readClass}" 
                 data-id="${notification.id}"
                 onclick="NotificationCenter.handleNotificationClick('${notification.id}')">
                <div class="flex gap-3">
                    <div class="shrink-0 mt-0.5">
                        <i class="fa-solid ${iconClass} text-lg"></i>
                    </div>
                    <div class="flex-1 min-w-0">
                        <div class="flex items-start justify-between gap-2">
                            <div class="font-medium text-sm text-slate-800 dark:text-slate-200 truncate">${this.escapeHtml(notification.title)}</div>
                            ${!notification.read ? '<span class="w-2 h-2 bg-indigo-500 rounded-full shrink-0 mt-1.5"></span>' : ''}
                        </div>
                        <div class="text-xs text-slate-500 dark:text-slate-400 mt-0.5 line-clamp-2">${this.escapeHtml(notification.message)}</div>
                        <div class="flex items-center justify-between mt-1.5">
                            <span class="text-[10px] text-slate-400 dark:text-slate-500">${timeAgo}</span>
                            <button onclick="event.stopPropagation(); NotificationCenter.deleteNotification('${notification.id}')" 
                                    class="text-[10px] text-slate-400 hover:text-red-500 transition-colors px-1.5 py-0.5 rounded hover:bg-red-50 dark:hover:bg-red-900/20">
                                <i class="fa-solid fa-trash"></i>
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        `;
    },

    handleNotificationClick(id) {
        this.markAsRead(id);
    },

    formatTimeAgo(timestamp) {
        const now = new Date();
        const date = new Date(timestamp);
        const diff = now - date;

        const seconds = Math.floor(diff / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        const days = Math.floor(hours / 24);

        if (seconds < 60) return '刚刚';
        if (minutes < 60) return `${minutes} 分钟前`;
        if (hours < 24) return `${hours} 小时前`;
        if (days < 7) return `${days} 天前`;
        
        return date.toLocaleDateString();
    },

    showDesktopNotification(title, body, source) {
        if (!('Notification' in window)) return;

        if (Notification.permission === 'granted') {
            this.createDesktopNotification(title, body, source);
        } else if (Notification.permission !== 'denied') {
            Notification.requestPermission().then(permission => {
                if (permission === 'granted') {
                    this.createDesktopNotification(title, body, source);
                }
            });
        }
    },

    createDesktopNotification(title, body, source) {
        try {
            const notification = new Notification(title, {
                body: body,
                icon: '/static/img/favicon.png',
                tag: source + '_' + Date.now()
            });

            notification.onclick = () => {
                window.focus();
                notification.close();
                const panel = document.getElementById('notification-panel');
                if (panel) {
                    this.togglePanel();
                }
            };

            setTimeout(() => notification.close(), 8000);
        } catch (e) {
            console.error('Desktop notification error:', e);
        }
    },

    requestDesktopPermission() {
        if (!('Notification' in window)) {
            return Promise.reject('Notifications not supported');
        }
        return Notification.requestPermission();
    },

    getDesktopPermissionStatus() {
        if (!('Notification' in window)) return 'unsupported';
        return Notification.permission;
    },

    playSound() {
        try {
            if (!_notificationAudio) {
                const AudioContext = window.AudioContext || window.webkitAudioContext;
                if (AudioContext) {
                    const ctx = new AudioContext();
                    const oscillator = ctx.createOscillator();
                    const gainNode = ctx.createGain();
                    
                    oscillator.connect(gainNode);
                    gainNode.connect(ctx.destination);
                    
                    oscillator.frequency.value = 800;
                    oscillator.type = 'sine';
                    gainNode.gain.setValueAtTime(0.3, ctx.currentTime);
                    gainNode.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + 0.3);
                    
                    oscillator.start(ctx.currentTime);
                    oscillator.stop(ctx.currentTime + 0.3);
                }
            }
        } catch (e) {
            console.error('Sound play error:', e);
        }
    },

    getSettings() {
        return JSON.parse(JSON.stringify(_notificationSettings));
    },

    updateSettings(settings) {
        _notificationSettings = this.mergeSettings(DefaultNotificationSettings, settings);
        this.saveSettings();
    },

    mergeSettings(defaults, overrides) {
        const result = {};
        for (const key in defaults) {
            if (typeof defaults[key] === 'object' && defaults[key] !== null && !Array.isArray(defaults[key])) {
                result[key] = this.mergeSettings(defaults[key], (overrides && overrides[key]) || {});
            } else {
                result[key] = (overrides && overrides[key] !== undefined) ? overrides[key] : defaults[key];
            }
        }
        return result;
    },

    escapeHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }
};

document.addEventListener('DOMContentLoaded', function() {
    NotificationCenter.init();
});

window.NotificationCenter = NotificationCenter;
window.NotificationSources = NotificationSources;
window.NotificationTypes = NotificationTypes;
window.SourceCategories = SourceCategories;
window.SourceLabels = SourceLabels;
