// Screen monitoring page - live screenshot viewer, FPS counter, slideshow, WebSocket

let isMonitoring = false;
let frameCount = 0;
let fpsInterval = null;
let lastFpsTime = Date.now();
let startTime = null;
let monitorTimer = null;

function startMonitor() {
    showLoading(true);

    fetch(`/agents/${agentId}/screen/start`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'}
    }).then(r => r.json()).then(data => {
        showLoading(false);

        if (data.success) {
            isMonitoring = true;
            startTime = Date.now();
            document.getElementById('start-btn').classList.add('hidden');
            document.getElementById('stop-btn').classList.remove('hidden');
            document.getElementById('status-indicator').innerHTML =
                '<span class="w-2 h-2 bg-emerald-500 rounded-full mr-2 animate-pulse"></span><span>监控中</span>';
            document.getElementById('play-pause-btn').innerHTML = '<i class="fa-solid fa-pause"></i>';
            document.getElementById('video-controls').style.opacity = '1';
            showToast('屏幕监控已启动');
            startFpsCounter();
            startMonitorTimer();
        } else {
            showError(data.error || '启动失败');
        }
    }).catch(err => {
        showLoading(false);
        console.error('Fetch error:', err);
        showError('网络错误: ' + err);
    });
}

function stopMonitor() {
    showLoading(true);
    fetch(`/agents/${agentId}/screen/stop`, {
        method: 'POST',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'}
    }).then(r => r.json()).then(data => {
        showLoading(false);
        if (data.success) {
            isMonitoring = false;
            document.getElementById('start-btn').classList.remove('hidden');
            document.getElementById('stop-btn').classList.add('hidden');
            document.getElementById('status-indicator').innerHTML =
                '<span class="w-2 h-2 bg-gray-400 rounded-full mr-2"></span><span>已停止</span>';
            document.getElementById('play-pause-btn').innerHTML = '<i class="fa-solid fa-play"></i>';
            stopFpsCounter();
            stopMonitorTimer();
            showToast('屏幕监控已停止');
        } else {
            showError(data.error || '停止失败');
        }
    }).catch(err => {
        showLoading(false);
        showError('网络错误: ' + err);
    });
}

function toggleMonitor() {
    if (isMonitoring) {
        stopMonitor();
    } else {
        startMonitor();
    }
}

function refreshScreenshot() {
    if (!isMonitoring) {
        showLoading(true);
        fetch(`/agents/${agentId}/screenshot`, {
            method: 'POST',
            headers: {'Content-Type': 'application/x-www-form-urlencoded'}
        }).then(r => r.json()).then(data => {
            showLoading(false);
            if (data.success) {
                showToast('截图请求已发送');
                setTimeout(fetchScreenshot, 2000);
            } else {
                showError(data.error || '请求失败');
            }
        }).catch(err => {
            showLoading(false);
            showError('网络错误: ' + err);
        });
    }
}

function fetchScreenshot() {
    fetch(`/agents/${agentId}/tasks?type=screenshot`)
    .then(r => r.json()).then(data => {
        if (data && data.length > 0) {
            const latestTask = data[0];
            if (latestTask.result && latestTask.status === 'completed') {
                displayScreenshot(latestTask.result);
            }
        }
    }).catch(err => console.error('Failed to fetch screenshot:', err));
}

function displayScreenshot(base64Data) {
    const container = document.getElementById('screenshot-container');

    let activeImg = document.querySelector('.screen-image-active');
    let inactiveImg = document.querySelector('.screen-image-inactive');

    if (!activeImg) {
        activeImg = document.createElement('img');
        activeImg.className = 'screen-image-active absolute inset-0 w-full h-full object-contain';
        activeImg.style.display = 'block';
        container.appendChild(activeImg);
    }
    if (!inactiveImg) {
        inactiveImg = document.createElement('img');
        inactiveImg.className = 'screen-image-inactive absolute inset-0 w-full h-full object-contain';
        inactiveImg.style.display = 'none';
        container.appendChild(inactiveImg);
    }

    inactiveImg.onload = function() {
        activeImg.style.display = 'none';
        inactiveImg.style.display = 'block';

        activeImg.classList.remove('screen-image-active');
        activeImg.classList.add('screen-image-inactive');
        inactiveImg.classList.remove('screen-image-inactive');
        inactiveImg.classList.add('screen-image-active');

        const temp = activeImg;
        activeImg = inactiveImg;
        inactiveImg = temp;

        frameCount++;
    };

    inactiveImg.onerror = function() {
        console.error('Failed to load screenshot image');
    };

    const mime = (base64Data && base64Data.indexOf('iVBOR') === 0) ? 'png' : 'jpeg';
    inactiveImg.src = 'data:image/' + mime + ';base64,' + base64Data;

    document.getElementById('last-update').textContent = '最后更新: ' + new Date().toLocaleString('zh-CN');
}

function showLoading(show) {
    const spinner = document.getElementById('loading-spinner');
    if (spinner) {
        spinner.style.opacity = show ? '1' : '0';
        spinner.style.pointerEvents = show ? 'auto' : 'none';
    }
}

function startFpsCounter() {
    stopFpsCounter();
    fpsInterval = setInterval(() => {
        const now = Date.now();
        const elapsed = now - lastFpsTime;
        if (elapsed >= 1000) {
            const fps = Math.round(frameCount * 1000 / elapsed);
            document.getElementById('fps-counter').textContent = `FPS: ${fps}`;
            frameCount = 0;
            lastFpsTime = now;
        }
    }, 100);
}

function stopFpsCounter() {
    if (fpsInterval) {
        clearInterval(fpsInterval);
        fpsInterval = null;
    }
    document.getElementById('fps-counter').textContent = 'FPS: 0';
    frameCount = 0;
}

function startMonitorTimer() {
    stopMonitorTimer();
    monitorTimer = setInterval(() => {
        if (startTime && isMonitoring) {
            const elapsed = Date.now() - startTime;
            const minutes = Math.floor(elapsed / 60000);
            const seconds = Math.floor((elapsed % 60000) / 1000);
            document.getElementById('time-display').textContent =
                `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
        }
    }, 1000);
}

function stopMonitorTimer() {
    if (monitorTimer) {
        clearInterval(monitorTimer);
        monitorTimer = null;
    }
}

function toggleFullscreen() {
    const container = document.querySelector('.bg-white.border.border-slate-200.rounded-3xl.shadow-sm.overflow-hidden');
    if (container) {
        if (!document.fullscreenElement) {
            container.requestFullscreen().catch(err => {
                console.error('Fullscreen error:', err);
            });
        } else {
            document.exitFullscreen();
        }
    }
}

function setupWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(protocol + '//' + window.location.host + '/ws');

    ws.onopen = function() {
        document.getElementById('ws-status').innerHTML =
            '<span class="w-2 h-2 bg-emerald-500 rounded-full mr-2"></span><span class="text-emerald-600">已连接</span>';
    };

    ws.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            if (data.type === 'screenshot' && data.agent_id === agentId) {
                displayScreenshot(data.data);
            }
        } catch (e) {
            console.error('Failed to parse WebSocket message:', e);
        }
    };

    ws.onerror = function(error) {
        console.error('WebSocket error:', error);
        document.getElementById('ws-status').innerHTML =
            '<span class="w-2 h-2 bg-red-400 rounded-full mr-2"></span><span class="text-red-600">错误</span>';
    };

    ws.onclose = function() {
        document.getElementById('ws-status').innerHTML =
            '<span class="w-2 h-2 bg-red-400 rounded-full mr-2"></span><span class="text-red-600">已断开</span>';
        setTimeout(setupWebSocket, 5000);
    };
}



function showError(message) {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'px-4 py-3 rounded-2xl shadow-xl flex items-center gap-x-3 text-sm bg-red-100 border border-red-300 text-red-700';
    toast.innerHTML = '<i class="fa-solid fa-exclamation-circle"></i>';
    const msgSpan2 = document.createElement('span');
    msgSpan2.textContent = message;
    toast.appendChild(msgSpan2);
    container.appendChild(toast);
    setTimeout(() => {
        toast.style.transition = 'all 0.3s ease';
        toast.style.opacity = '0';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

setupWebSocket();
