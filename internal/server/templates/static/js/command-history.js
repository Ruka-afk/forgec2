// Command History and Autocomplete Module
if (typeof window.CommandHistory === 'undefined') {
window.CommandHistory = (function() {
    const MAX_HISTORY = 100;
    const STORAGE_KEY = 'forgec2_cmd_history';

    let history = [];
    let historyIndex = -1;
    let tempInput = '';

    function init() {
        loadHistory();
    }

    function loadHistory() {
        try {
            const stored = localStorage.getItem(STORAGE_KEY);
            history = stored ? JSON.parse(stored) : [];
        } catch (e) {
            console.error('Failed to load command history:', e);
            history = [];
        }
    }

    function saveHistory() {
        try {
            const unique = [...new Set(history)].slice(-MAX_HISTORY);
            localStorage.setItem(STORAGE_KEY, JSON.stringify(unique));
        } catch (e) {
            console.error('Failed to save command history:', e);
        }
    }

    function add(cmd) {
        if (!cmd || cmd.trim() === '') return;
        history = history.filter(c => c !== cmd);
        history.push(cmd);
        if (history.length > MAX_HISTORY) {
            history = history.slice(-MAX_HISTORY);
        }
        saveHistory();
        resetIndex();
    }

    function up(currentInput) {
        if (history.length === 0) return currentInput;
        if (historyIndex === -1) {
            tempInput = currentInput;
        }
        if (historyIndex < history.length - 1) {
            historyIndex++;
            return history[history.length - 1 - historyIndex];
        }
        return history[0];
    }

    function down(currentInput) {
        if (history.length === 0 || historyIndex === -1) return currentInput;
        if (historyIndex > 0) {
            historyIndex--;
            return history[history.length - 1 - historyIndex];
        }
        historyIndex = -1;
        return tempInput;
    }

    function resetIndex() {
        historyIndex = -1;
        tempInput = '';
    }

    function search(query) {
        if (!query || query.trim() === '') return history.slice(-10);
        const lowerQuery = query.toLowerCase();
        return history.filter(cmd => cmd.toLowerCase().includes(lowerQuery)).slice(-10);
    }

    function getAll() {
        return history.slice();
    }

    function clear() {
        history = [];
        saveHistory();
        resetIndex();
    }

    return {
        init,
        add,
        up,
        down,
        search,
        getAll,
        clear,
        resetIndex
    };
})();
}

if (typeof window.AutoComplete === 'undefined') {
window.AutoComplete = (function() {
    const WINDOWS_COMMANDS = [
        'whoami', 'ipconfig', 'net user', 'net group', 'net localgroup',
        'systeminfo', 'tasklist', 'taskkill', 'sc query', 'reg query',
        'dir', 'type', 'copy', 'move', 'del', 'mkdir', 'rmdir',
        'ping', 'tracert', 'netstat', 'arp', 'nslookup',
        'powershell', 'cmd', 'wmic', 'schtasks', 'wevtutil'
    ];

    const LINUX_COMMANDS = [
        'whoami', 'id', 'ifconfig', 'ip', 'uname', 'hostname',
        'ps', 'top', 'kill', 'ls', 'cat', 'cp', 'mv', 'rm', 'mkdir',
        'chmod', 'chown', 'grep', 'find', 'awk', 'sed',
        'netstat', 'ss', 'route', 'ping', 'traceroute',
        'sudo', 'su', 'passwd', 'useradd', 'userdel'
    ];

    const COMMON_COMMANDS = [
        'help', 'exit', 'clear', 'history', 'upload', 'download',
        'shell', 'ps', 'screenshot', 'keylogger', 'clipboard'
    ];

    let suggestions = [];
    let selectedIndex = -1;

    function getSuggestions(input, osType = 'windows') {
        if (!input || input.trim() === '') return [];

        const lowerInput = input.toLowerCase();
        const commands = osType === 'linux' ?
            [...LINUX_COMMANDS, ...COMMON_COMMANDS] :
            [...WINDOWS_COMMANDS, ...COMMON_COMMANDS];

        const prefixMatches = commands.filter(cmd =>
            cmd.toLowerCase().startsWith(lowerInput)
        );
        const containsMatches = commands.filter(cmd =>
            cmd.toLowerCase().includes(lowerInput) && !prefixMatches.includes(cmd)
        );

        return [...prefixMatches, ...containsMatches].slice(0, 10);
    }

    function show(container, input, onSelect) {
        suggestions = getSuggestions(input);
        selectedIndex = -1;

        if (suggestions.length === 0) {
            hide(container);
            return;
        }

        container.innerHTML = suggestions.map((cmd, idx) => `
            <div class="autocomplete-item px-3 py-2 hover:bg-indigo-50 cursor-pointer text-sm"
                 data-index="${idx}" data-command="${cmd}">
                <i class="fa-solid fa-terminal w-4 text-slate-400"></i>
                <span class="ml-2">${highlightMatch(cmd, input)}</span>
            </div>
        `).join('');

        container.classList.remove('hidden');

        container.querySelectorAll('.autocomplete-item').forEach(item => {
            item.addEventListener('click', () => {
                const cmd = item.dataset.command;
                onSelect(cmd);
                hide(container);
            });
        });
    }

    function hide(container) {
        container.classList.add('hidden');
        container.innerHTML = '';
        suggestions = [];
        selectedIndex = -1;
    }

    function navigate(direction) {
        if (suggestions.length === 0) return null;

        if (direction === 'up') {
            selectedIndex = selectedIndex > 0 ? selectedIndex - 1 : suggestions.length - 1;
        } else {
            selectedIndex = selectedIndex < suggestions.length - 1 ? selectedIndex + 1 : 0;
        }

        return suggestions[selectedIndex];
    }

    function getSelected() {
        if (selectedIndex >= 0 && selectedIndex < suggestions.length) {
            return suggestions[selectedIndex];
        }
        return null;
    }

    function highlightMatch(text, query) {
        if (!query) return text;
        const regex = new RegExp(`(${escapeRegex(query)})`, 'gi');
        return text.replace(regex, '<strong class="text-indigo-600">$1</strong>');
    }

    function escapeRegex(str) {
        return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }

    return {
        getSuggestions,
        show,
        hide,
        navigate,
        getSelected
    };
})();
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => window.CommandHistory.init());
} else {
    window.CommandHistory.init();
}