// Enhanced File Browser Module
const FileBrowser = (function() {
    let currentPath = '';
    let history = [];
    let historyIndex = -1;
    
    // Initialize
    function init() {
        currentPath = '';
        history = [];
        historyIndex = -1;
    }
    
    // Navigate to directory
    function navigate(path, pushHistory = true) {
        if (pushHistory && currentPath !== path) {
            if (historyIndex < history.length - 1) {
                history = history.slice(0, historyIndex + 1);
            }
            history.push(currentPath);
            historyIndex = history.length - 1;
        }
        
        currentPath = path;
        loadDirectory(path);
    }
    
    // Load directory contents
    function loadDirectory(path) {
        // This would call the agent's ls command
        // Show loading
        const container = document.getElementById('file-list');
        if (container) {
            container.innerHTML = '<div class="text-center py-8 text-slate-500"><i class="fa-solid fa-spinner fa-spin text-2xl"></i><p class="mt-2">加载中...</p></div>';
        }
        
        // Trigger agent command (would be implemented in actual file browser page)
        // fetch(`/agents/${agentId}/command`, { ... })
    }
    
    // Go back in history
    function back() {
        if (historyIndex >= 0) {
            const prevPath = history[historyIndex];
            historyIndex--;
            navigate(prevPath, false);
        }
    }
    
    // Go forward in history
    function forward() {
        if (historyIndex < history.length - 2) {
            historyIndex++;
            const nextPath = history[historyIndex + 1];
            navigate(nextPath, false);
        }
    }
    
    // Go up one directory
    function up() {
        if (!currentPath || currentPath === '/' || currentPath.endsWith(':\\')) {
            return; // Already at root
        }
        
        let parentPath;
        if (currentPath.includes('/')) {
            // Unix path
            const parts = currentPath.split('/').filter(p => p);
            parts.pop();
            parentPath = '/' + parts.join('/');
        } else {
            // Windows path
            const parts = currentPath.split('\\').filter(p => p);
            parts.pop();
            parentPath = parts.join('\\');
            if (parts.length === 1) {
                parentPath += '\\'; // Keep C:\ format
            }
        }
        
        navigate(parentPath);
    }
    
    // Refresh current directory
    function refresh() {
        loadDirectory(currentPath);
    }
    
    // Upload file via drag & drop
    function handleDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        
        const files = e.dataTransfer.files;
        if (files.length > 0) {
            uploadFiles(files);
        }
    }
    
    // Upload multiple files
    function uploadFiles(files) {
        for (let i = 0; i < files.length; i++) {
            uploadFile(files[i]);
        }
    }
    
    // Upload single file
    function uploadFile(file) {
        const reader = new FileReader();
        reader.onload = function(e) {
            const base64 = e.target.result.split(',')[1];
            // Send to agent (would be implemented)
        };
        reader.readAsDataURL(file);
    }
    
    // Download file
    function download(filePath, fileName) {
        // Trigger download task (would be implemented)
    }
    
    // Delete file
    function deleteFile(filePath) {
        if (!confirm(`确定要删除 ${filePath} 吗？`)) {
            return;
        }
        // Send delete command (would be implemented)
    }
    
    // Rename file
    function rename(oldPath, oldName) {
        const newName = prompt('输入新名称:', oldName);
        if (!newName || newName === oldName) {
            return;
        }
        // Send rename command (would be implemented)
    }
    
    // Create new folder
    function createFolder() {
        const name = prompt('输入文件夹名称:');
        if (!name) return;
        
        const newPath = currentPath + (currentPath.includes('/') ? '/' : '\\') + name;
        // Send mkdir command (would be implemented)
    }
    
    // Search files
    function search(pattern) {
        if (!pattern) {
            refresh();
            return;
        }
        // Send find command (would be implemented)
    }
    
    // Show context menu
    function showContextMenu(e, filePath, fileName, isDir) {
        e.preventDefault();
        
        const menu = document.createElement('div');
        menu.className = 'fixed bg-slate-800 border border-slate-600 rounded-lg shadow-lg py-1 z-50';
        menu.style.left = e.pageX + 'px';
        menu.style.top = e.pageY + 'px';
        
        const items = [
            { icon: 'fa-download', label: '下载', action: () => download(filePath, fileName), show: !isDir },
            { icon: 'fa-trash', label: '删除', action: () => deleteFile(filePath), show: true },
            { icon: 'fa-pen', label: '重命名', action: () => rename(filePath, fileName), show: true },
            { icon: 'fa-copy', label: '复制路径', action: () => copyToClipboard(filePath), show: true },
        ];
        
        items.forEach(item => {
            if (!item.show) return;
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
        
        // Close on click outside
        setTimeout(() => {
            document.addEventListener('click', function handler() {
                if (menu.parentNode) {
                    menu.parentNode.removeChild(menu);
                }
                document.removeEventListener('click', handler);
            });
        }, 100);
    }
    
    // Copy to clipboard
    function copyToClipboard(text) {
        navigator.clipboard.writeText(text).then(() => {
            // Show toast notification
        });
    }
    
    // Render file list
    function renderFileList(files, container) {
        container.innerHTML = '';
        
        if (files.length === 0) {
            container.innerHTML = '<div class="text-center py-8 text-slate-500"><i class="fa-solid fa-folder-open text-4xl mb-2"></i><p>空目录</p></div>';
            return;
        }
        
        // Sort: directories first, then files
        files.sort((a, b) => {
            if (a.isDir && !b.isDir) return -1;
            if (!a.isDir && b.isDir) return 1;
            return a.name.localeCompare(b.name);
        });
        
        files.forEach(file => {
            const row = document.createElement('div');
            row.className = 'flex items-center gap-3 px-3 py-2 hover:bg-slate-700/50 rounded cursor-pointer';
            row.ondblclick = () => {
                if (file.isDir) {
                    navigate(file.path);
                } else {
                    download(file.path, file.name);
                }
            };
            row.oncontextmenu = (e) => showContextMenu(e, file.path, file.name, file.isDir);
            
            const icon = file.isDir ? 'fa-folder text-yellow-400' : getFileIcon(file.name);
            const size = file.isDir ? '-' : formatBytes(file.size);
            const modified = file.modified ? new Date(file.modified).toLocaleString('zh-CN') : '-';
            
            row.innerHTML = `
                <i class="fa-solid ${icon} w-5 text-center"></i>
                <span class="flex-1 text-slate-300">${escapeHtml(file.name)}</span>
                <span class="text-slate-500 text-xs w-20 text-right">${size}</span>
                <span class="text-slate-600 text-xs w-40 text-right">${modified}</span>
            `;
            
            container.appendChild(row);
        });
    }
    
    // Get file icon based on extension
    function getFileIcon(fileName) {
        const ext = fileName.split('.').pop().toLowerCase();
        const iconMap = {
            'txt': 'fa-file-lines text-slate-400',
            'log': 'fa-file-lines text-slate-400',
            'pdf': 'fa-file-pdf text-red-400',
            'doc': 'fa-file-word text-blue-400',
            'docx': 'fa-file-word text-blue-400',
            'xls': 'fa-file-excel text-green-400',
            'xlsx': 'fa-file-excel text-green-400',
            'ppt': 'fa-file-powerpoint text-orange-400',
            'pptx': 'fa-file-powerpoint text-orange-400',
            'jpg': 'fa-file-image text-purple-400',
            'jpeg': 'fa-file-image text-purple-400',
            'png': 'fa-file-image text-purple-400',
            'gif': 'fa-file-image text-purple-400',
            'zip': 'fa-file-zipper text-yellow-400',
            'rar': 'fa-file-zipper text-yellow-400',
            '7z': 'fa-file-zipper text-yellow-400',
            'exe': 'fa-file-code text-green-400',
            'dll': 'fa-file-code text-green-400',
            'js': 'fa-file-code text-yellow-400',
            'html': 'fa-file-code text-orange-400',
            'css': 'fa-file-code text-blue-400',
            'py': 'fa-file-code text-blue-400',
            'go': 'fa-file-code text-cyan-400',
        };
        
        return iconMap[ext] || 'fa-file text-slate-400';
    }
    
    // Format bytes to human readable
    function formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
    }
    
    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
    
    return {
        init,
        navigate,
        back,
        forward,
        up,
        refresh,
        handleDrop,
        uploadFiles,
        download,
        deleteFile,
        rename,
        createFolder,
        search,
        renderFileList,
        showContextMenu
    };
})();

// Export to global scope
window.FileBrowser = FileBrowser;
