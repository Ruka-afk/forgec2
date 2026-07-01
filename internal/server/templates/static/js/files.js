// File browser page - directory listing, upload, download, preview, delete with chunked transfers

let currentPath = 'C:\\';
let deleteTargetPath = '';
let uploadTargetPath = '';

document.addEventListener('DOMContentLoaded', function() {
    if (!document.getElementById('current-path') || typeof agentId === 'undefined') return;
    listDir(currentPath);
});

function listDir(path) {
    currentPath = path;
    document.getElementById('current-path').value = path;

    document.getElementById('loading-state').classList.remove('hidden');
    document.getElementById('file-table').classList.add('hidden');
    document.getElementById('empty-state').classList.add('hidden');

    fetch(`/agents/${agentId}/files/ls`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: `path=${encodeURIComponent(path)}`
    }).then(r => r.json()).then(data => {
        if (data.success) {
            setTimeout(() => fetchListResult(data.task_id), 2000);
        } else {
            showError(data.error || __t('Request failed'));
        }
    }).catch(err => {
        showError(__t('Network error:') + ' ' + err);
    });
}

function fetchListResult(taskId) {
    fetch(`/agents/${agentId}/tasks/${taskId}`)
        .then(r => r.json())
        .then(data => {
            if (data.status === 'completed') {
                renderFileList(data.result);
            } else if (data.status === 'failed') {
                showError(data.error || __t('Failed to get file list'));
            } else {
                setTimeout(() => fetchListResult(taskId), 2000);
            }
        }).catch(err => {
            showError(__t('Network error:') + ' ' + err);
        });
}

function renderFileList(rawResult) {
    document.getElementById('loading-state').classList.add('hidden');

    if (!rawResult || rawResult.trim() === '') {
        document.getElementById('empty-state').classList.remove('hidden');
        return;
    }

    const lines = rawResult.split('\n');
    const fileList = document.getElementById('file-list');
    fileList.innerHTML = '';

    let files = [];
    for (let i = 2; i < lines.length; i++) {
        const line = lines[i].trim();
        if (!line) continue;
        const parts = line.split('\t');
        if (parts.length >= 4) {
            files.push({
                type: parts[0],
                name: parts[1],
                size: parts[2],
                modified: parts[3]
            });
        }
    }

    if (files.length === 0) {
        document.getElementById('empty-state').classList.remove('hidden');
        return;
    }

    document.getElementById('file-table').classList.remove('hidden');

    files.sort((a, b) => {
        if (a.type === 'DIR' && b.type !== 'DIR') return -1;
        if (a.type !== 'DIR' && b.type === 'DIR') return 1;
        return a.name.localeCompare(b.name);
    });

    for (const file of files) {
        const fullPath = currentPath.endsWith('\\') ? currentPath + file.name : currentPath + '\\' + file.name;
        const isDir = file.type === 'DIR';

        const row = document.createElement('tr');
        row.className = 'hover:bg-slate-50 transition-colors';
        row.innerHTML = `
            <td class="py-3 px-4">
                ${isDir
                    ? '<i class="fa-solid fa-folder text-indigo-500"></i>'
                    : '<i class="fa-solid fa-file text-slate-400"></i>'}
            </td>
            <td class="py-3 px-4">
                <span class="text-sm ${isDir ? 'text-indigo-600 font-medium hover:underline cursor-pointer' : 'text-slate-700'}"
                      ${isDir ? `onclick="goTo('${escapeJs(fullPath)}')"` : ''}>
                    ${escapeHtml(file.name)}
                </span>
            </td>
            <td class="py-3 px-4 text-sm text-slate-500">
                ${formatSize(file.size)}
            </td>
            <td class="py-3 px-4 text-sm text-slate-500">
                ${escapeHtml(file.modified)}
            </td>
            <td class="py-3 px-4 text-right">
                <div class="flex items-center justify-end gap-x-2">
                    ${isDir ? `
                        <button onclick="goTo('${escapeJs(fullPath)}')"
                                class="text-indigo-600 hover:text-indigo-800 text-xs px-2 py-1 rounded hover:bg-indigo-50">
                            <i class="fa-solid fa-folder-open"></i>
                        </button>
                    ` : `
                        <button onclick="readFile('${escapeJs(fullPath)}', '${escapeJs(file.name)}')"
                                class="text-slate-600 hover:text-slate-800 text-xs px-2 py-1 rounded hover:bg-slate-100" title="${__t('View content')}">
                            <i class="fa-solid fa-eye"></i>
                        </button>
                        <button onclick="uploadFile('${escapeJs(fullPath)}', '${escapeJs(file.name)}')"
                                class="text-emerald-600 hover:text-emerald-800 text-xs px-2 py-1 rounded hover:bg-emerald-50" title="${__t('Upload to server')}">
                            <i class="fa-solid fa-upload"></i>
                        </button>
                        <button onclick="downloadToLocal('${escapeJs(fullPath)}')"
                                class="text-blue-600 hover:text-blue-800 text-xs px-2 py-1 rounded hover:bg-blue-50" title="${__t('Download to local')}">
                            <i class="fa-solid fa-download"></i>
                        </button>
                    `}
                    <button onclick="deleteFile('${escapeJs(fullPath)}', '${escapeJs(file.name)}')"
                            class="text-red-600 hover:text-red-800 text-xs px-2 py-1 rounded hover:bg-red-50" title="${__t('Delete')}">
                        <i class="fa-solid fa-trash"></i>
                    </button>
                </div>
            </td>
        `;
        fileList.appendChild(row);
    }
}

function goTo(path) {
    listDir(path);
}

function navigateToPath() {
    const path = document.getElementById('current-path').value.trim();
    if (path) {
        listDir(path);
    }
}

function refreshList() {
    listDir(currentPath);
}

function readFile(path, name) {
    fetch(`/agents/${agentId}/files/read`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: `path=${encodeURIComponent(path)}`
    }).then(r => r.json()).then(data => {
        if (data.success) {
            showToast(__t('Reading file...'));
            setTimeout(() => fetchReadResult(data.task_id, name), 2000);
        } else {
            showError(data.error || __t('Request failed'));
        }
    });
}

function fetchReadResult(taskId, name) {
    fetch(`/agents/${agentId}/tasks/${taskId}`)
        .then(r => r.json())
        .then(data => {
            if (data.status === 'completed') {
                showPreview(name, data.result);
            } else if (data.status === 'failed') {
                showError(data.error || __t('Failed to read file'));
            } else {
                setTimeout(() => fetchReadResult(taskId, name), 2000);
            }
        });
}

function showPreview(name, content) {
    document.getElementById('preview-title').textContent = name;
    document.getElementById('preview-content').innerHTML = `
        <pre class="bg-slate-50 rounded-xl p-4 text-sm font-mono text-slate-700 whitespace-pre-wrap overflow-auto">${escapeHtml(content)}</pre>
    `;
    document.getElementById('preview-modal').classList.remove('hidden');
}

function closePreview() {
    document.getElementById('preview-modal').classList.add('hidden');
}

function deleteFile(path, name) {
    deleteTargetPath = path;
    document.getElementById('delete-file-name').textContent = name;
    document.getElementById('delete-modal').classList.remove('hidden');
}

function closeDeleteModal() {
    document.getElementById('delete-modal').classList.add('hidden');
    deleteTargetPath = '';
}

function confirmFileDelete() {
    if (!deleteTargetPath) return;
    fetch(`/agents/${agentId}/files/delete`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: `path=${encodeURIComponent(deleteTargetPath)}`
    }).then(r => r.json()).then(data => {
        closeDeleteModal();
        if (data.success) {
            showToast(__t('Delete request sent'));
            setTimeout(() => refreshList(), 3000);
        } else {
            showError(data.error || __t('Request failed'));
        }
    });
}

function uploadFile(path, name) {
    uploadTargetPath = path;
    document.getElementById('upload-file-name').textContent = name;
    document.getElementById('upload-modal').classList.remove('hidden');
}

function closeUploadModal() {
    document.getElementById('upload-modal').classList.add('hidden');
    uploadTargetPath = '';
}

function confirmUpload() {
    if (!uploadTargetPath) return;
    fetch(`/agents/${agentId}/files/upload`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: `path=${encodeURIComponent(uploadTargetPath)}`
    }).then(r => r.json()).then(data => {
        closeUploadModal();
        if (data.success) {
            showToast(__t('Upload request sent, check task results later'));
        } else {
            showError(data.error || __t('Request failed'));
        }
    });
}

function formatSize(size) {
    if (size === '-' || !size) return '-';
    const bytes = parseInt(size);
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
    return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
}

function escapeJs(text) {
    return text.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}

function showError(msg) {
    document.getElementById('loading-state').classList.add('hidden');
    document.getElementById('file-table').classList.add('hidden');
    document.getElementById('empty-state').classList.remove('hidden');
    document.getElementById('empty-state').innerHTML = `
        <i class="fa-solid fa-exclamation-circle text-4xl text-red-300 mb-3"></i>
        <p class="text-red-500">${escapeHtml(msg)}</p>
    `;
}

function uploadLocalFileToAgent() {
    const input = document.createElement('input');
    input.type = 'file';
    input.onchange = async (e) => {
        const file = e.target.files[0];
        if (!file) return;
        const targetPath = currentPath.endsWith('\\') ? currentPath + file.name : currentPath + '\\' + file.name;
        if (!confirm(__tf('Upload local file {0} ({1}) to {2}?', file.name, formatSize(file.size), targetPath))) return;
        await chunkedUploadToAgent(file, targetPath);
    };
    input.click();
}

async function chunkedUploadToAgent(file, targetPath) {
    const chunkSize = 1024 * 1024;
    const totalChunks = Math.ceil(file.size / chunkSize);
    let uploaded = 0;

    for (let i = 0; i < totalChunks; i++) {
        const start = i * chunkSize;
        const chunk = file.slice(start, start + chunkSize);

        const formData = new FormData();
        formData.append('target_path', targetPath);
        formData.append('offset', start);
        formData.append('file', chunk);

        try {
            const response = await fetch(`/agents/${agentId}/upload`, {
                method: 'POST',
                body: formData
            });
            const data = await response.json();
            if (!data.success) {
                showError(data.error || __t('Upload chunk failed'));
                return;
            }
            uploaded += chunk.size;
            const percent = Math.round((uploaded / file.size) * 100);
            showToast(__tf('Upload progress: {0}%', percent));
        } catch (err) {
            showError(__t('Upload error:') + ' ' + err.message);
            return;
        }
    }
    showToast(__t('File upload complete!'));
    setTimeout(() => refreshList(), 1500);
}

async function downloadToLocal(path) {
    const chunkSize = 1024 * 1024;
    let offset = 0;
    const chunks = [];
    const fileName = path.split('\\').pop() || 'downloaded_file';

    showToast(__t('Starting chunked download...'));

    while (true) {
        const resp = await fetch(`/agents/${agentId}/files/download`, {
            method: 'POST',
            headers: {'Content-Type': 'application/x-www-form-urlencoded'},
            body: `path=${encodeURIComponent(path)}&offset=${offset}&size=${chunkSize}`
        });
        const data = await resp.json();
        if (!data.success) {
            showError(data.error || __t('Download chunk request failed'));
            return;
        }
        const taskResult = await pollDownloadChunk(data.task_id);
        if (!taskResult || taskResult.status !== 'completed' || !taskResult.result) {
            break;
        }
        try {
            const binary = atob(taskResult.result);
            const bytes = new Uint8Array(binary.length);
            for (let j = 0; j < binary.length; j++) {
                bytes[j] = binary.charCodeAt(j);
            }
            chunks.push(bytes);
            offset += chunkSize;
            showToast(__tf('Downloaded {0} MB', Math.floor(offset / 1024 / 1024)));
            if (bytes.length < chunkSize) break;
        } catch (e) {
            showError(__t('Decode chunk failed'));
            return;
        }
    }

    let totalLen = 0;
    chunks.forEach(c => totalLen += c.length);
    const allBytes = new Uint8Array(totalLen);
    let pos = 0;
    chunks.forEach(c => {
        allBytes.set(c, pos);
        pos += c.length;
    });
    const blob = new Blob([allBytes]);
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = fileName;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    showToast(__t('Download complete'));
}

async function pollDownloadChunk(taskId) {
    return new Promise((resolve) => {
        const check = () => {
            fetch(`/agents/${agentId}/tasks/${taskId}`)
                .then(r => r.json())
                .then(data => {
                    if (data.status === 'completed' || data.status === 'failed') {
                        resolve(data);
                    } else {
                        setTimeout(check, 1500);
                    }
                })
                .catch(() => resolve(null));
        };
        check();
    });
}
