// Registry Editor Module (Windows)
const RegistryEditor = (function() {
    let currentKey = '';
    
    // Common registry roots
    const REG_ROOTS = [
        { name: 'HKEY_LOCAL_MACHINE', short: 'HKLM' },
        { name: 'HKEY_CURRENT_USER', short: 'HKCU' },
        { name: 'HKEY_CLASSES_ROOT', short: 'HKCR' },
        { name: 'HKEY_USERS', short: 'HKU' },
        { name: 'HKEY_CURRENT_CONFIG', short: 'HKCC' }
    ];
    
    // Initialize
    function init() {
        currentKey = 'HKLM\\SOFTWARE';
        loadRoots();
    }
    
    // Load registry roots
    function loadRoots() {
        const container = document.getElementById('registry-tree');
        if (!container) return;
        
        container.innerHTML = '';
        
        REG_ROOTS.forEach(root => {
            const node = createTreeNode(root.short, root.name, true);
            container.appendChild(node);
        });
    }
    
    // Create tree node
    function createTreeNode(label, fullPath, isRoot = false) {
        const div = document.createElement('div');
        div.className = 'registry-node';
        
        const item = document.createElement('div');
        item.className = 'flex items-center gap-2 px-2 py-1 hover:bg-slate-700/50 rounded cursor-pointer';
        item.innerHTML = `
            <i class="fa-solid fa-chevron-right text-slate-500 text-xs w-3"></i>
            <i class="fa-solid fa-folder text-yellow-400 w-4"></i>
            <span class="text-slate-300 text-sm">${label}</span>
        `;
        
        item.addEventListener('click', () => {
            toggleNode(div, fullPath);
        });
        
        div.appendChild(item);
        return div;
    }
    
    // Toggle node expansion
    function toggleNode(nodeDiv, fullPath) {
        const childContainer = nodeDiv.querySelector(':scope > .registry-children');
        const icon = nodeDiv.querySelector('i.fa-chevron-right, i.fa-chevron-down');
        
        if (childContainer) {
            // Toggle existing
            childContainer.classList.toggle('hidden');
            if (icon) {
                icon.classList.toggle('fa-chevron-right');
                icon.classList.toggle('fa-chevron-down');
            }
        } else {
            // Load children
            loadSubKeys(fullPath, nodeDiv);
            if (icon) {
                icon.classList.remove('fa-chevron-right');
                icon.classList.add('fa-chevron-down');
            }
        }
    }
    
    // Load subkeys
    function loadSubKeys(parentPath, nodeDiv) {
        // This would call the agent's registry command
        // Create children container
        const children = document.createElement('div');
        children.className = 'registry-children ml-4 space-y-1';
        
        // Show loading
        children.innerHTML = '<div class="text-slate-500 text-xs p-2">加载中...</div>';
        nodeDiv.appendChild(children);
        
        // Would call: fetch(`/agents/${agentId}/reg/list?key=${parentPath}`)
    }
    
    // Load registry values
    function loadValues(keyPath) {
        currentKey = keyPath;
        
        const container = document.getElementById('registry-values');
        if (!container) return;
        
        container.innerHTML = '<div class="text-center py-8 text-slate-500"><i class="fa-solid fa-spinner fa-spin text-2xl"></i><p class="mt-2">加载中...</p></div>';
        
        // Would call: fetch(`/agents/${agentId}/reg/get?key=${keyPath}`)
    }
    
    // Render values table
    function renderValues(values, container) {
        if (values.length === 0) {
            container.innerHTML = '<div class="text-slate-500 text-sm p-4">无键值</div>';
            return;
        }
        
        const table = document.createElement('table');
        table.className = 'w-full text-sm';
        
        table.innerHTML = `
            <thead class="bg-slate-700">
                <tr>
                    <th class="px-3 py-2 text-left text-slate-300">名称</th>
                    <th class="px-3 py-2 text-left text-slate-300">类型</th>
                    <th class="px-3 py-2 text-left text-slate-300">数据</th>
                    <th class="px-3 py-2 text-left text-slate-300">操作</th>
                </tr>
            </thead>
            <tbody></tbody>
        `;
        
        const tbody = table.querySelector('tbody');
        
        values.forEach(val => {
            const row = document.createElement('tr');
            row.className = 'hover:bg-slate-700/50';
            row.innerHTML = `
                <td class="px-3 py-2 text-slate-300">${escapeHtml(val.name)}</td>
                <td class="px-3 py-2 text-slate-400 text-xs">${val.type}</td>
                <td class="px-3 py-2 text-slate-300 font-mono text-xs">${escapeHtml(val.data)}</td>
                <td class="px-3 py-2">
                    <button onclick="editValue('${val.name}')" class="text-blue-400 hover:text-blue-300 mr-2">
                        <i class="fa-solid fa-pen"></i>
                    </button>
                    <button onclick="deleteValue('${val.name}')" class="text-red-400 hover:text-red-300">
                        <i class="fa-solid fa-trash"></i>
                    </button>
                </td>
            `;
            tbody.appendChild(row);
        });
        
        container.innerHTML = '';
        container.appendChild(table);
    }
    
    // Edit value
    function editValue(name) {
        const newData = prompt('输入新值:');
        if (newData === null) return;
        
        // Would call: fetch(`/agents/${agentId}/reg/set`, { ... })
    }
    
    // Delete value
    function deleteValue(name) {
        if (!confirm(`确定要删除键值 ${name} 吗？`)) return;
        
        // Would call: fetch(`/agents/${agentId}/reg/delete`, { ... })
    }
    
    // Create new key
    function createKey() {
        const name = prompt('输入新键名称:');
        if (!name) return;
        
        const newPath = currentKey + '\\' + name;
        // Would call agent command
    }
    
    // Create new value
    function createValue() {
        const name = prompt('输入值名称:');
        if (!name) return;
        
        const type = prompt('输入类型 (REG_SZ, REG_DWORD, REG_BINARY):', 'REG_SZ');
        if (!type) return;
        
        const data = prompt('输入数据:');
        if (data === null) return;
        
        // Would call agent command
    }
    
    // Search registry
    function search(query) {
        if (!query) return;
        
        // Would call agent command
    }
    
    // Export registry key
    function exportKey(keyPath) {
        // Would call agent command to export as .reg file
    }
    
    // Show context menu
    function showContextMenu(e, keyPath) {
        e.preventDefault();
        
        const menu = document.createElement('div');
        menu.className = 'fixed bg-slate-800 border border-slate-600 rounded-lg shadow-lg py-1 z-50';
        menu.style.left = e.pageX + 'px';
        menu.style.top = e.pageY + 'px';
        
        const items = [
            { icon: 'fa-plus', label: '新建键', action: createKey },
            { icon: 'fa-plus-circle', label: '新建值', action: createValue },
            { icon: 'fa-download', label: '导出', action: () => exportKey(keyPath) },
            { icon: 'fa-copy', label: '复制路径', action: () => copyToClipboard(keyPath) },
        ];
        
        items.forEach(item => {
            const div = document.createElement('div');
            div.className = 'px-4 py-2 hover:bg-slate-700 cursor-pointer flex items-center gap-2 text-sm text-slate-300';
            div.innerHTML = `<i class="fa-solid ${item.icon} w-4"></i> ${item.label}`;
            div.addEventListener('click', () => {
                item.action();
                document.body.removeChild(menu);
            });
            menu.appendChild(div);
        });
        
        document.body.appendChild(menu);
        
        setTimeout(() => {
            document.addEventListener('click', function handler() {
                if (menu.parentNode) {
                    menu.parentNode.removeChild(menu);
                }
                document.removeEventListener('click', handler);
            });
        }, 100);
    }
    
    function copyToClipboard(text) {
        navigator.clipboard.writeText(text);
    }
    
    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
    
    return {
        init,
        loadValues,
        renderValues,
        editValue,
        deleteValue,
        createKey,
        createValue,
        search,
        showContextMenu
    };
})();

// Export to global scope
window.RegistryEditor = RegistryEditor;
