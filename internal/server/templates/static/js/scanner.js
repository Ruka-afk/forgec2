(function initScannerPage() {
    const portModeEl = document.getElementById('scan-port-mode');
    const scanForm = document.getElementById('scan-form');
    if (!portModeEl || !scanForm) return;

    portModeEl.addEventListener('change', function() {
    const customRange = document.getElementById('custom-port-range');
    if (customRange) customRange.classList.toggle('hidden', this.value !== 'custom');
});

    scanForm.addEventListener('submit', async function(e) {
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
        const btn = this.querySelector('button[type="submit"]');
        try {
            btn.disabled = true;
            const result = await apiFetch('/api/scan', {method: 'POST', body: formData});
            if (result.success) {
                showToast(__('Scan task created'), 'success');
                pollScanResults(result.task_id);
            } else {
                showToast(result.error || __('Create failed'), 'error');
            }
        } catch (error) {
            showToast(__('Network error'), 'error');
        } finally {
            btn.disabled = false;
        }
    });
})();

let currentTaskId = null;
function pollScanResults(taskId) {
    currentTaskId = taskId;
    const poll = async () => {
        try {
            const data = await apiFetch(`/api/scan/results/${taskId}`);
            if (data.results && data.results.length > 0) displayScanResults(data.results);
            setTimeout(poll, 3000);
        } catch (error) {
            showToast(__('Failed to poll scan results'), 'error');
        }
    };
    poll();
}

function displayScanResults(results) {
    const tbody = document.getElementById('scan-results-body');
    const openPorts = results.filter(r => r.state === 'open');
    document.getElementById('result-count').textContent = __tf('{0} open ports', openPorts.length);
    if (results.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" class="text-center py-8 text-slate-400"><i class="fa-solid fa-inbox text-3xl mb-2"></i><p>' + __('No scan results') + '</p></td></tr>';
        return;
    }
    tbody.innerHTML = results.map(r => '<tr class="border-b border-slate-100 hover:bg-slate-50 transition-colors">' +
        '<td class="py-3 px-4 font-mono font-semibold">' + r.port + '</td>' +
        '<td class="py-3 px-4 uppercase">' + r.protocol + '</td>' +
        '<td class="py-3 px-4"><span class="px-2 py-1 rounded-lg text-xs font-medium ' +
            (r.state === 'open' ? 'bg-emerald-100 text-emerald-700' : r.state === 'closed' ? 'bg-slate-100 text-slate-600' : 'bg-amber-100 text-amber-700') + '">' + r.state + '</span></td>' +
        '<td class="py-3 px-4 font-mono">' + (r.service || '-') + '</td>' +
        '<td class="py-3 px-4 text-xs">' + (r.version || '-') + '</td>' +
        '<td class="py-3 px-4 font-mono text-xs truncate max-w-xs">' + (r.banner || '-') + '</td></tr>').join('');
}

function exportResults() {
    if (!currentTaskId) { showToast(__('Please run a scan first'), 'warning'); return; }
    window.open('/api/scan/export/' + currentTaskId, '_blank');
}
