// 切换端口配置模式
document.getElementById('scan-port-mode').addEventListener('change', function() {
    const customRange = document.getElementById('custom-port-range');
    if (this.value === 'custom') {
        customRange.classList.remove('hidden');
    } else {
        customRange.classList.add('hidden');
    }
});

// 提交扫描任务
document.getElementById('scan-form').addEventListener('submit', async function(e) {
    e.preventDefault();
    
    const agentId = document.getElementById('scan-agent').value;
    const target = document.getElementById('scan-target').value;
    const scanType = document.getElementById('scan-type').value;
    const portMode = document.getElementById('scan-port-mode').value;
    
    const formData = new FormData();
    formData.append('agent_id', agentId);
    formData.append('target', target);
    formData.append('scan_type', scanType);
    
    if (portMode === 'custom') {
        formData.append('port_range', document.getElementById('scan-ports').value);
    } else if (portMode === 'top100') {
        formData.append('top_ports', '100');
    } else {
        formData.append('top_ports', '1000');
    }
    
    try {
        const response = await fetch('/api/scan', {
            method: 'POST',
            body: formData
        });
        
        const result = await response.json();
        if (result.success) {
            showToast('扫描任务已创建', 'success');
            // 开始轮询结果
            pollScanResults(result.task_id);
        } else {
            showToast(result.error || '创建失败', 'error');
        }
    } catch (error) {
        showToast('网络错误', 'error');
    }
});

// 轮询扫描结果
let currentTaskId = null;
function pollScanResults(taskId) {
    currentTaskId = taskId;
    
    const poll = async () => {
        try {
            const response = await fetch(`/api/scan/results/${taskId}`);
            const data = await response.json();
            
            if (data.results && data.results.length > 0) {
                displayScanResults(data.results);
            }
            
            // 继续轮询（可根据任务状态调整）
            setTimeout(poll, 3000);
        } catch (error) {
            showToast('扫描结果轮询失败', 'error');
        }
    };
    
    poll();
}

// 显示扫描结果
function displayScanResults(results) {
    const tbody = document.getElementById('scan-results-body');
    const openPorts = results.filter(r => r.state === 'open');
    
    document.getElementById('result-count').textContent = `共 ${openPorts.length} 个开放端口`;
    
    if (results.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="6" class="text-center py-8 text-slate-400">
                    <i class="fa-solid fa-inbox text-3xl mb-2"></i>
                    <p>暂无扫描结果</p>
                </td>
            </tr>
        `;
        return;
    }
    
    tbody.innerHTML = results.map(r => `
        <tr class="border-b border-slate-100 hover:bg-slate-50 transition-colors">
            <td class="py-3 px-4 font-mono font-semibold">${r.port}</td>
            <td class="py-3 px-4 uppercase">${r.protocol}</td>
            <td class="py-3 px-4">
                <span class="px-2 py-1 rounded-lg text-xs font-medium ${
                    r.state === 'open' ? 'bg-emerald-100 text-emerald-700' :
                    r.state === 'closed' ? 'bg-slate-100 text-slate-600' :
                    'bg-amber-100 text-amber-700'
                }">${r.state}</span>
            </td>
            <td class="py-3 px-4 font-mono">${r.service || '-'}</td>
            <td class="py-3 px-4 text-xs">${r.version || '-'}</td>
            <td class="py-3 px-4 font-mono text-xs truncate max-w-xs">${r.banner || '-'}</td>
        </tr>
    `).join('');
}

// 导出结果
function exportResults() {
    if (!currentTaskId) {
        showToast('请先执行扫描', 'warning');
        return;
    }
    window.open(`/api/scan/export/${currentTaskId}`, '_blank');
}
