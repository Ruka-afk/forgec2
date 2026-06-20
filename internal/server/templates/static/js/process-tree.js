// Process Tree Visualization Module
const ProcessTree = (function() {
    
    // Parse process list text into tree structure
    function parseProcessList(text) {
        if (!text) return [];
        
        const lines = text.trim().split('\n');
        const processes = [];
        
        // Skip header line if present
        const startIdx = lines[0].includes('PID') || lines[0].includes('Process') ? 1 : 0;
        
        for (let i = startIdx; i < lines.length; i++) {
            const line = lines[i].trim();
            if (!line) continue;
            
            // Try to parse different formats
            // Format 1: PID  PPID  Name  User
            // Format 2: PID  Name  User
            const parts = line.split(/\s+/);
            
            if (parts.length >= 3) {
                const proc = {
                    pid: parts[0],
                    ppid: parts.length >= 4 ? parts[1] : '0',
                    name: parts.length >= 4 ? parts[2] : parts[1],
                    user: parts.length >= 4 ? parts[3] : parts[2],
                    children: []
                };
                processes.push(proc);
            }
        }
        
        return buildTree(processes);
    }
    
    // Build tree from flat list
    function buildTree(processes) {
        const map = {};
        const roots = [];
        
        // Create map
        processes.forEach(p => {
            map[p.pid] = p;
        });
        
        // Build tree
        processes.forEach(p => {
            if (p.ppid === '0' || p.ppid === '1' || !map[p.ppid]) {
                roots.push(p);
            } else {
                if (map[p.ppid]) {
                    map[p.ppid].children.push(p);
                } else {
                    roots.push(p);
                }
            }
        });
        
        return roots;
    }
    
    // Render tree to HTML
    function renderTree(roots, container) {
        container.innerHTML = '';
        
        if (roots.length === 0) {
            container.innerHTML = '<div class="text-slate-500 text-sm p-4">暂无进程数据，请先执行 ps 命令</div>';
            return;
        }
        
        const ul = document.createElement('ul');
        ul.className = 'process-tree space-y-1';
        
        roots.forEach(root => {
            renderNode(root, ul, 0);
        });
        
        container.appendChild(ul);
    }
    
    // Render single node
    function renderNode(proc, parent, depth) {
        const li = document.createElement('li');
        li.className = 'process-node';
        li.style.marginLeft = (depth * 20) + 'px';
        
        // Determine process type by color
        let colorClass = 'text-slate-300';
        let iconClass = 'fa-microchip';
        
        if (proc.user === 'SYSTEM' || proc.user === 'root') {
            colorClass = 'text-red-400';
            iconClass = 'fa-shield-halved';
        } else if (proc.name.toLowerCase().includes('explorer') || 
                   proc.name.toLowerCase().includes('desktop')) {
            colorClass = 'text-blue-400';
            iconClass = 'fa-window-maximize';
        } else if (proc.name.toLowerCase().includes('cmd') || 
                   proc.name.toLowerCase().includes('powershell') ||
                   proc.name.toLowerCase().includes('bash') ||
                   proc.name.toLowerCase().includes('sh')) {
            colorClass = 'text-green-400';
            iconClass = 'fa-terminal';
        }
        
        const nodeDiv = document.createElement('div');
        nodeDiv.className = 'flex items-center gap-2 px-3 py-2 hover:bg-slate-700/50 rounded cursor-pointer transition-colors';
        nodeDiv.innerHTML = `
            <i class="fa-solid ${iconClass} ${colorClass} w-4"></i>
            <span class="${colorClass} font-mono text-sm">${escapeHtml(proc.name)}</span>
            <span class="text-slate-500 text-xs">(PID: ${proc.pid})</span>
            <span class="text-slate-600 text-xs">[${escapeHtml(proc.user)}]</span>
            <div class="flex-1"></div>
            <button class="text-xs px-2 py-1 bg-red-600/20 text-red-400 hover:bg-red-600/40 rounded" 
                    onclick="killProcess('${proc.pid}', '${escapeHtml(proc.name)}')"
                    title="终止进程">
                <i class="fa-solid fa-skull"></i>
            </button>
        `;
        
        // Toggle children on click
        nodeDiv.addEventListener('click', (e) => {
            if (e.target.tagName === 'BUTTON' || e.target.closest('button')) {
                return; // Don't toggle when clicking kill button
            }
            const childUl = li.querySelector(':scope > ul');
            if (childUl) {
                childUl.classList.toggle('hidden');
                const icon = nodeDiv.querySelector('i');
                if (childUl.classList.contains('hidden')) {
                    icon.className = icon.className.replace('fa-chevron-down', 'fa-chevron-right');
                } else {
                    icon.className = icon.className.replace('fa-chevron-right', 'fa-chevron-down');
                }
            }
        });
        
        li.appendChild(nodeDiv);
        
        // Render children
        if (proc.children && proc.children.length > 0) {
            const childUl = document.createElement('ul');
            childUl.className = 'space-y-1';
            proc.children.forEach(child => {
                renderNode(child, childUl, depth + 1);
            });
            li.appendChild(childUl);
        }
        
        parent.appendChild(li);
    }
    
    // Search processes
    function search(query, container) {
        if (!query) {
            // Show all
            container.querySelectorAll('.process-node').forEach(node => {
                node.style.display = '';
            });
            return;
        }
        
        const lowerQuery = query.toLowerCase();
        container.querySelectorAll('.process-node').forEach(node => {
            const name = node.querySelector('span').textContent.toLowerCase();
            const pid = node.textContent.toLowerCase();
            if (name.includes(lowerQuery) || pid.includes(lowerQuery)) {
                node.style.display = '';
                // Show parents
                let parent = node.parentElement;
                while (parent && parent !== container) {
                    if (parent.tagName === 'LI') {
                        parent.style.display = '';
                    }
                    parent = parent.parentElement;
                }
            } else {
                node.style.display = 'none';
            }
        });
    }
    
    // Filter by user
    function filterByUser(user, container) {
        if (!user) {
            container.querySelectorAll('.process-node').forEach(node => {
                node.style.display = '';
            });
            return;
        }
        
        const lowerUser = user.toLowerCase();
        container.querySelectorAll('.process-node').forEach(node => {
            const userSpan = node.querySelector('span:nth-child(3)');
            if (userSpan && userSpan.textContent.toLowerCase().includes(lowerUser)) {
                node.style.display = '';
            } else {
                node.style.display = 'none';
            }
        });
    }
    
    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
    
    return {
        parseProcessList,
        buildTree,
        renderTree,
        search,
        filterByUser
    };
})();

// Export to global scope
window.ProcessTree = ProcessTree;
