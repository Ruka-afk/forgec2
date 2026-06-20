// Command History and Autocomplete Module
const CommandHistory = (function() {
    const MAX_HISTORY = 100;
    const STORAGE_KEY = 'forgec2_cmd_history';
    
    let history = [];
    let historyIndex = -1;
    let tempInput = '';
    
    // Initialize
    function init() {
        loadHistory();
    }
    
    // Load from localStorage
    function loadHistory() {
        try {
            const stored = localStorage.getItem(STORAGE_KEY);
            history = stored ? JSON.parse(stored) : [];
        } catch (e) {
            console.error('Failed to load command history:', e);
            history = [];
        }
    }
    
    // Save to localStorage
    function saveHistory() {
        try {
            // Deduplicate and limit
            const unique = [...new Set(history)].slice(-MAX_HISTORY);
            localStorage.setItem(STORAGE_KEY, JSON.stringify(unique));
        } catch (e) {
            console.error('Failed to save command history:', e);
        }
    }
    
    // Add command to history
    function add(cmd) {
        if (!cmd || cmd.trim() === '') return;
        
        // Remove duplicate if exists
        history = history.filter(c => c !== cmd);
        
        // Add to end
        history.push(cmd);
        
        // Limit size
        if (history.length > MAX_HISTORY) {
            history = history.slice(-MAX_HISTORY);
        }
        
        saveHistory();
        resetIndex();
    }
    
    // Navigate up (older)
    function up(currentInput) {
        if (history.length === 0) return currentInput;
        
        // Save current input if at the end
        if (historyIndex === -1) {
            tempInput = currentInput;
        }
        
        if (historyIndex < history.length - 1) {
            historyIndex++;
            return history[history.length - 1 - historyIndex];
        }
        
        return history[0];
    }
    
    // Navigate down (newer)
    function down(currentInput) {
        if (history.length === 0 || historyIndex === -1) return currentInput;
        
        if (historyIndex > 0) {
            historyIndex--;
            return history[history.length - 1 - historyIndex];
        } else {
            historyIndex = -1;
            return tempInput;
        }
    }
    
    // Reset index
    function resetIndex() {
        historyIndex = -1;
        tempInput = '';
    }
    
    // Search history (fuzzy match)
    function search(query) {
        if (!query || query.trim() === '') return history.slice(-10);
        
        const lowerQuery = query.toLowerCase();
        return history.filter(cmd => cmd.toLowerCase().includes(lowerQuery)).slice(-10);
    }
    
    // Get all history
    function getAll() {
        return history.slice();
    }
    
    // Clear history
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

// Autocomplete Module
const AutoComplete = (function() {
    // Common commands by OS
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
    
    // Get suggestions based on input
    function getSuggestions(input, osType = 'windows') {
        if (!input || input.trim() === '') return [];
        
        const lowerInput = input.toLowerCase();
        const commands = osType === 'linux' ? 
            [...LINUX_COMMANDS, ...COMMON_COMMANDS] : 
            [...WINDOWS_COMMANDS, ...COMMON_COMMANDS];
        
        // Exact prefix match first
        const prefixMatches = commands.filter(cmd => 
            cmd.toLowerCase().startsWith(lowerInput)
        );
        
        // Then contains match
        const containsMatches = commands.filter(cmd => 
            cmd.toLowerCase().includes(lowerInput) && !prefixMatches.includes(cmd)
        );
        
        return [...prefixMatches, ...containsMatches].slice(0, 10);
    }
    
    // Show suggestions UI
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
        
        // Add click handlers
        container.querySelectorAll('.autocomplete-item').forEach(item => {
            item.addEventListener('click', () => {
                const cmd = item.dataset.command;
                onSelect(cmd);
                hide(container);
            });
        });
    }
    
    // Hide suggestions
    function hide(container) {
        container.classList.add('hidden');
        container.innerHTML = '';
        suggestions = [];
        selectedIndex = -1;
    }
    
    // Navigate suggestions
    function navigate(direction) {
        if (suggestions.length === 0) return null;
        
        if (direction === 'up') {
            selectedIndex = selectedIndex > 0 ? selectedIndex - 1 : suggestions.length - 1;
        } else {
            selectedIndex = selectedIndex < suggestions.length - 1 ? selectedIndex + 1 : 0;
        }
        
        return suggestions[selectedIndex];
    }
    
    // Get selected suggestion
    function getSelected() {
        if (selectedIndex >= 0 && selectedIndex < suggestions.length) {
            return suggestions[selectedIndex];
        }
        return null;
    }
    
    // Highlight matching text
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

// Export to global scope
window.CommandHistory = CommandHistory;
window.AutoComplete = AutoComplete;

// Initialize on DOM ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => CommandHistory.init());
} else {
    CommandHistory.init();
}
