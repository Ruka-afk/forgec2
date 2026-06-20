// Keyboard Shortcuts System
const KeyboardShortcuts = (function() {
    
    // Shortcut registry
    const shortcuts = new Map();
    
    // Modifier keys
    const MODIFIERS = {
        CTRL: 'ctrl',
        ALT: 'alt',
        SHIFT: 'shift',
        META: 'meta'
    };
    
    // Parse shortcut string (e.g., "ctrl+s", "alt+shift+f")
    function parseShortcut(shortcutStr) {
        const parts = shortcutStr.toLowerCase().split('+');
        const modifiers = {
            ctrl: false,
            alt: false,
            shift: false,
            meta: false
        };
        let key = '';
        
        parts.forEach(part => {
            if (MODIFIERS[part]) {
                modifiers[part] = true;
            } else {
                key = part;
            }
        });
        
        return { key, modifiers };
    }
    
    // Check if event matches shortcut
    function matchesShortcut(event, shortcut) {
        const parsed = parseShortcut(shortcut);
        
        return event.key.toLowerCase() === parsed.key &&
               event.ctrlKey === parsed.modifiers.ctrl &&
               event.altKey === parsed.modifiers.alt &&
               event.shiftKey === parsed.modifiers.shift &&
               event.metaKey === parsed.modifiers.meta;
    }
    
    // Register a shortcut
    function register(shortcut, description, handler, context = 'global') {
        if (!shortcuts.has(context)) {
            shortcuts.set(context, new Map());
        }
        
        shortcuts.get(context).set(shortcut, {
            description,
            handler,
            enabled: true
        });
        
        console.log(`[Shortcuts] Registered: ${shortcut} - ${description}`);
    }
    
    // Unregister a shortcut
    function unregister(shortcut, context = 'global') {
        if (shortcuts.has(context)) {
            shortcuts.get(context).delete(shortcut);
        }
    }
    
    // Enable/disable a shortcut
    function setEnabled(shortcut, enabled, context = 'global') {
        if (shortcuts.has(context) && shortcuts.get(context).has(shortcut)) {
            shortcuts.get(context).get(shortcut).enabled = enabled;
        }
    }
    
    // Get all shortcuts for a context
    function getShortcuts(context = 'global') {
        return shortcuts.get(context) || new Map();
    }
    
    // Handle keyboard event
    function handleKeyDown(event) {
        // Skip if typing in input/textarea/contenteditable
        if (event.target.tagName === 'INPUT' || 
            event.target.tagName === 'TEXTAREA' ||
            event.target.isContentEditable ||
            event.target.type === 'text' ||
            event.target.type === 'password' ||
            event.target.type === 'email' ||
            event.target.type === 'search' ||
            event.target.type === 'tel' ||
            event.target.type === 'url' ||
            event.target.type === 'number') {
            return;
        }
        
        // Check all contexts
        for (const [context, contextShortcuts] of shortcuts) {
            for (const [shortcut, config] of contextShortcuts) {
                if (config.enabled && matchesShortcut(event, shortcut)) {
                    event.preventDefault();
                    event.stopPropagation();
                    
                    try {
                        config.handler(event);
                    } catch (err) {
                        console.error(`[Shortcuts] Error in handler for ${shortcut}:`, err);
                    }
                    
                    return;
                }
            }
        }
    }
    
    // Show help modal
    function showHelp() {
        const modal = document.createElement('div');
        modal.className = 'fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50';
        modal.id = 'shortcuts-help-modal';
        
        const allShortcuts = [];
        for (const [context, contextShortcuts] of shortcuts) {
            for (const [shortcut, config] of contextShortcuts) {
                allShortcuts.push({
                    shortcut,
                    description: config.description,
                    context,
                    enabled: config.enabled
                });
            }
        }
        
        modal.innerHTML = `
            <div class="bg-slate-800 rounded-lg shadow-2xl max-w-2xl w-full mx-4 max-h-[80vh] overflow-hidden">
                <div class="flex items-center justify-between p-4 border-b border-slate-700">
                    <h2 class="text-xl font-semibold text-white">
                        <i class="fa-solid fa-keyboard mr-2"></i>键盘快捷键
                    </h2>
                    <button onclick="document.getElementById('shortcuts-help-modal').remove()" 
                            class="text-slate-400 hover:text-white">
                        <i class="fa-solid fa-times"></i>
                    </button>
                </div>
                <div class="p-4 overflow-y-auto max-h-[calc(80vh-80px)]">
                    <div class="space-y-4">
                        ${allShortcuts.map(s => `
                            <div class="flex items-center justify-between p-3 bg-slate-700 rounded-lg ${!s.enabled ? 'opacity-50' : ''}">
                                <div class="flex-1">
                                    <div class="text-sm text-slate-300">${s.description}</div>
                                    <div class="text-xs text-slate-500 mt-1">${s.context}</div>
                                </div>
                                <div class="flex gap-1 ml-4">
                                    ${s.shortcut.split('+').map(key => `
                                        <kbd class="px-2 py-1 bg-slate-900 text-slate-300 rounded text-xs font-mono">${key.toUpperCase()}</kbd>
                                    `).join('<span class="text-slate-500">+</span>')}
                                </div>
                            </div>
                        `).join('')}
                    </div>
                    <div class="mt-4 p-3 bg-slate-900 rounded-lg text-sm text-slate-400">
                        <i class="fa-solid fa-info-circle mr-2"></i>
                        提示：快捷键在输入框中不可用
                    </div>
                </div>
            </div>
        `;
        
        document.body.appendChild(modal);
        
        // Close on escape
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                modal.remove();
            }
        });
    }
    
    // Initialize keyboard listener
    function init() {
        document.addEventListener('keydown', handleKeyDown);
        
        // Register default shortcuts
        register('?', '显示快捷键帮助', showHelp);
        register('escape', '关闭弹窗', () => {
            const modal = document.getElementById('shortcuts-help-modal');
            if (modal) modal.remove();
        });
        
        console.log('[Shortcuts] Initialized');
    }
    
    // Cleanup
    function destroy() {
        document.removeEventListener('keydown', handleKeyDown);
        shortcuts.clear();
    }
    
    return {
        init,
        destroy,
        register,
        unregister,
        setEnabled,
        getShortcuts,
        showHelp,
        MODIFIERS
    };
})();

// Global Navigation Shortcuts
const NavigationShortcuts = (function() {
    
    const pages = {
        'g': {
            'd': { url: '/dashboard', name: '控制台' },
            'a': { url: '/agents', name: 'Agent 管理' },
            'l': { url: '/listeners', name: '监听器' },
            't': { url: '/tasks', name: '任务历史' },
            'c': { url: '/credentials', name: '凭据收集' },
            's': { url: '/shell', name: 'Shell 终端' },
            'f': { url: '/files', name: '文件管理' },
            'r': { url: '/report', name: '报告生成' },
        }
    };
    
    let firstKey = null;
    let firstKeyTimeout = null;
    
    function init() {
        document.addEventListener('keydown', handleKeyDown);
    }
    
    function handleKeyDown(event) {
        // Skip if typing in input
        if (event.target.tagName === 'INPUT' || 
            event.target.tagName === 'TEXTAREA' ||
            event.target.isContentEditable ||
            event.target.type === 'text' ||
            event.target.type === 'password' ||
            event.target.type === 'email' ||
            event.target.type === 'search' ||
            event.target.type === 'tel' ||
            event.target.type === 'url' ||
            event.target.type === 'number') {
            return;
        }
        
        const key = event.key.toLowerCase();
        
        // Two-key navigation (g + key)
        if (key === 'g') {
            firstKey = 'g';
            
            if (firstKeyTimeout) {
                clearTimeout(firstKeyTimeout);
            }
            
            firstKeyTimeout = setTimeout(() => {
                firstKey = null;
            }, 1000);
            
            event.preventDefault();
            return;
        }
        
        if (firstKey === 'g' && pages.g[key]) {
            const page = pages.g[key];
            window.location.href = page.url;
            firstKey = null;
            event.preventDefault();
        }
    }
    
    function getPages() {
        return pages;
    }
    
    return {
        init,
        getPages
    };
})();

// Shell Terminal Shortcuts
const ShellShortcuts = (function() {
    
    function init() {
        if (!document.getElementById('shell-terminal')) {
            return;
        }
        
        // Register shell-specific shortcuts
        KeyboardShortcuts.register('ctrl+l', '清空终端', clearTerminal, 'shell');
        KeyboardShortcuts.register('ctrl+c', '取消命令', cancelCommand, 'shell');
        KeyboardShortcuts.register('ctrl+u', '清空输入', clearInput, 'shell');
        KeyboardShortcuts.register('ctrl+k', '删除到行尾', deleteToEnd, 'shell');
        KeyboardShortcuts.register('ctrl+a', '光标到行首', cursorToStart, 'shell');
        KeyboardShortcuts.register('ctrl+e', '光标到行尾', cursorToEnd, 'shell');
        KeyboardShortcuts.register('ctrl+r', '搜索历史', searchHistory, 'shell');
        
        console.log('[ShellShortcuts] Initialized');
    }
    
    function clearTerminal() {
        const output = document.getElementById('shell-output');
        if (output) {
            output.innerHTML = '';
        }
    }
    
    function cancelCommand() {
        const input = document.getElementById('cmd-input');
        if (input) {
            input.value = '';
            input.focus();
        }
    }
    
    function clearInput() {
        const input = document.getElementById('cmd-input');
        if (input) {
            input.value = '';
        }
    }
    
    function deleteToEnd() {
        const input = document.getElementById('cmd-input');
        if (input) {
            const start = input.selectionStart;
            input.value = input.value.substring(0, start);
        }
    }
    
    function cursorToStart() {
        const input = document.getElementById('cmd-input');
        if (input) {
            input.setSelectionRange(0, 0);
        }
    }
    
    function cursorToEnd() {
        const input = document.getElementById('cmd-input');
        if (input) {
            const len = input.value.length;
            input.setSelectionRange(len, len);
        }
    }
    
    function searchHistory() {
        // Trigger command history search
        if (typeof CommandHistory !== 'undefined') {
            const input = document.getElementById('cmd-input');
            if (input) {
                const query = input.value;
                const results = CommandHistory.search(query);
                // Show search results
            }
        }
    }
    
    return {
        init
    };
})();

// Export to global scope
window.KeyboardShortcuts = KeyboardShortcuts;
window.NavigationShortcuts = NavigationShortcuts;
window.ShellShortcuts = ShellShortcuts;

// Auto-initialize on DOM ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        KeyboardShortcuts.init();
        NavigationShortcuts.init();
        ShellShortcuts.init();
    });
} else {
    KeyboardShortcuts.init();
    NavigationShortcuts.init();
    ShellShortcuts.init();
}
