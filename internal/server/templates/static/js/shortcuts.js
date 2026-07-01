const DEFAULT_SHORTCUTS = {
    new_item: {
        key: 'n',
        ctrl: true,
        shift: false,
        alt: false,
        label: '新建',
        description: '根据当前页面创建新项'
    },
    save: {
        key: 's',
        ctrl: true,
        shift: false,
        alt: false,
        label: '保存',
        description: '保存当前内容'
    },
    show_shortcuts: {
        key: '/',
        ctrl: true,
        shift: false,
        alt: false,
        label: '快捷键帮助',
        description: '显示所有可用快捷键'
    },
    close_modal: {
        key: 'Escape',
        ctrl: false,
        shift: false,
        alt: false,
        label: '关闭',
        description: '关闭模态框或下拉列表'
    },
    refresh: {
        key: 'F5',
        ctrl: false,
        shift: false,
        alt: false,
        label: '刷新',
        description: '刷新当前页面'
    },
    toggle_lock: {
        key: 'l',
        ctrl: true,
        shift: true,
        alt: false,
        label: '锁定/解锁 Agent',
        description: '锁定或解锁当前 Agent'
    },
    global_search: {
        key: 'k',
        ctrl: true,
        shift: false,
        alt: false,
        label: '全局搜索',
        description: '打开全局搜索'
    }
};

const SHORTCUT_STORAGE_KEY = 'forgec2_shortcuts';

function loadShortcuts() {
    try {
        const saved = localStorage.getItem(SHORTCUT_STORAGE_KEY);
        if (saved) {
            const custom = JSON.parse(saved);
            const result = {};
            Object.keys(DEFAULT_SHORTCUTS).forEach(key => {
                result[key] = { ...DEFAULT_SHORTCUTS[key], ...(custom[key] || {}) };
            });
            return result;
        }
    } catch (e) {
        console.error('Failed to load shortcuts:', e);
    }
    return { ...DEFAULT_SHORTCUTS };
}

function saveShortcuts(customShortcuts) {
    try {
        localStorage.setItem(SHORTCUT_STORAGE_KEY, JSON.stringify(customShortcuts));
    } catch (e) {
        console.error('Failed to save shortcuts:', e);
    }
}

function resetShortcuts() {
    try {
        localStorage.removeItem(SHORTCUT_STORAGE_KEY);
    } catch (e) {
        console.error('Failed to reset shortcuts:', e);
    }
    return { ...DEFAULT_SHORTCUTS };
}

function formatShortcut(shortcut, platform = 'auto') {
    const isMac = platform === 'mac' || (platform === 'auto' && navigator.platform.toUpperCase().indexOf('MAC') >= 0);
    const parts = [];
    
    if (shortcut.ctrl) {
        parts.push(isMac ? 'Cmd' : 'Ctrl');
    }
    if (shortcut.shift) {
        parts.push('Shift');
    }
    if (shortcut.alt) {
        parts.push(isMac ? 'Opt' : 'Alt');
    }
    
    const keyDisplay = shortcut.key === 'Escape' ? 'Esc' : 
                       shortcut.key === 'F5' ? 'F5' : 
                       shortcut.key === '/' ? '?' : 
                       shortcut.key.toUpperCase();
    parts.push(keyDisplay);
    
    return parts.join(' + ');
}

function parseShortcutString(str) {
    const parts = str.split('+').map(p => p.trim());
    const result = {
        key: '',
        ctrl: false,
        shift: false,
        alt: false
    };
    
    for (const part of parts) {
        const lower = part.toLowerCase();
        if (lower === 'ctrl' || lower === 'cmd') {
            result.ctrl = true;
        } else if (lower === 'shift') {
            result.shift = true;
        } else if (lower === 'alt' || lower === 'opt') {
            result.alt = true;
        } else {
            if (part === 'Esc') {
                result.key = 'Escape';
            } else if (part === '?') {
                result.key = '/';
            } else {
                result.key = part.length === 1 ? part.toLowerCase() : part;
            }
        }
    }
    
    return result;
}

function matchShortcut(event, shortcut) {
    const ctrlMatch = shortcut.ctrl === (event.ctrlKey || event.metaKey);
    const shiftMatch = shortcut.shift === event.shiftKey;
    const altMatch = shortcut.alt === event.altKey;
    
    let keyMatch = false;
    if (shortcut.key === 'Escape') {
        keyMatch = event.key === 'Escape';
    } else if (shortcut.key.startsWith('F')) {
        keyMatch = event.key === shortcut.key;
    } else if (shortcut.key.length === 1) {
        keyMatch = event.key.toLowerCase() === shortcut.key.toLowerCase();
    } else {
        keyMatch = event.key === shortcut.key;
    }
    
    return ctrlMatch && shiftMatch && altMatch && keyMatch;
}

const Shortcuts = {
    DEFAULT: DEFAULT_SHORTCUTS,
    load: loadShortcuts,
    save: saveShortcuts,
    reset: resetShortcuts,
    format: formatShortcut,
    parse: parseShortcutString,
    match: matchShortcut
};

window.Shortcuts = Shortcuts;
