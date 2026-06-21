function generateReport() {
    const data = {
        start_date: document.getElementById('start-date').value,
        end_date: document.getElementById('end-date').value,
        include: {
            agents: document.getElementById('include-agents').checked,
            tasks: document.getElementById('include-tasks').checked,
            creds: document.getElementById('include-creds').checked,
            screenshots: document.getElementById('include-screenshots').checked,
            audit: document.getElementById('include-audit').checked
        },
        format: document.querySelector('input[name="format"]:checked').value
    };

    showToast('正在生成报告...', 'success');

    fetch('/api/report/generate', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    })
    .then(r => {
        if (data.format === 'json') {
            return r.json().then(json => {
                const blob = new Blob([JSON.stringify(json, null, 2)], {type: 'application/json'});
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'forgec2_report_' + new Date().toISOString().slice(0,10) + '.json';
                a.click();
                URL.revokeObjectURL(url);
                showToast('报告已下载', 'success');
            });
        } else {
            return r.text().then(html => {
                const blob = new Blob([html], {type: 'text/html'});
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'forgec2_report_' + new Date().toISOString().slice(0,10) + '.html';
                a.click();
                URL.revokeObjectURL(url);
                showToast('报告已下载', 'success');
            });
        }
    })
    .catch(err => {
        console.error('Failed to generate report:', err);
        showToast('生成报告失败', 'error');
    });
}

function previewReport() {
    const data = {
        start_date: document.getElementById('start-date').value,
        end_date: document.getElementById('end-date').value,
        include: {
            agents: document.getElementById('include-agents').checked,
            tasks: document.getElementById('include-tasks').checked,
            creds: document.getElementById('include-creds').checked,
            screenshots: document.getElementById('include-screenshots').checked,
            audit: document.getElementById('include-audit').checked
        },
        format: 'html'
    };

    showToast('正在生成预览...', 'success');

    fetch('/api/report/generate', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    })
    .then(r => r.text())
    .then(html => {
        const win = window.open('', '_blank');
        win.document.write(html);
        win.document.close();
    })
    .catch(err => {
        console.error('Failed to preview report:', err);
        showToast('预览失败', 'error');
    });
}
