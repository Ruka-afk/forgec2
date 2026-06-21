let ws = null;
let reconnectAttempts = 0;
const maxReconnectAttempts = 5;

document.addEventListener('DOMContentLoaded', function() {
    loadMessages();
    connectWebSocket();

    document.getElementById('chat-form').addEventListener('submit', function(e) {
        e.preventDefault();
        sendMessage();
    });

    document.getElementById('message-input').addEventListener('keydown', function(e) {
        if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
            e.preventDefault();
            sendMessage();
        }
    });
});

function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/chat?user_id=${encodeURIComponent(currentUsername)}`;

    ws = new WebSocket(wsUrl);

    ws.onopen = function() {
        reconnectAttempts = 0;
    };

    ws.onmessage = function(event) {
        try {
            const msg = JSON.parse(event.data);
            appendMessage(msg);
            scrollToBottom();
        } catch (e) {
            console.error('Failed to parse message:', e);
        }
    };

    ws.onerror = function(error) {
        console.error('WebSocket error:', error);
    };

    ws.onclose = function() {
        if (reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++;
            setTimeout(connectWebSocket, 2000 * reconnectAttempts);
        }
    };
}

function loadMessages() {
    fetch('/api/chat/messages')
        .then(r => r.json())
        .then(data => {
            const container = document.getElementById('messages-container');
            container.innerHTML = '';

            if (data.messages && data.messages.length > 0) {
                data.messages.forEach(msg => appendMessage(msg));
            } else {
                container.innerHTML = `
                    <div class="text-center text-slate-400 text-sm py-8">
                        <i class="fa-solid fa-comments text-4xl mb-2"></i>
                        <p>暂无消息，开始聊天吧！</p>
                    </div>
                `;
            }
            scrollToBottom();
        })
        .catch(err => {
            console.error('Failed to load messages:', err);
        });
}

function appendMessage(msg) {
    const container = document.getElementById('messages-container');
    const isMe = msg.user === currentUsername;

    const msgEl = document.createElement('div');
    msgEl.className = `flex ${isMe ? 'justify-end' : 'justify-start'}`;

    const time = new Date(msg.timestamp || msg.created_at).toLocaleTimeString('zh-CN', {
        hour: '2-digit',
        minute: '2-digit'
    });

    msgEl.innerHTML = `
        <div class="max-w-[70%] ${isMe ? 'order-2' : 'order-1'}">
            <div class="flex items-center gap-2 mb-1 ${isMe ? 'justify-end' : 'justify-start'}">
                <span class="text-sm font-medium ${isMe ? 'text-indigo-600' : 'text-slate-700'}">${escapeHtml(msg.user)}</span>
                <span class="text-xs text-slate-400">${time}</span>
            </div>
            <div class="${isMe ? 'bg-indigo-600 text-white' : 'bg-white border border-slate-200'} rounded-xl px-4 py-2.5 shadow-sm">
                <p class="text-sm whitespace-pre-wrap break-words">${escapeHtml(msg.message)}</p>
            </div>
        </div>
    `;

    container.appendChild(msgEl);
}

function sendMessage() {
    const input = document.getElementById('message-input');
    const message = input.value.trim();

    if (!message) return;

    fetch('/api/chat/send', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: message })
    })
    .then(r => r.json())
    .then(data => {
        if (data.message === 'sent') {
            input.value = '';
        }
    })
    .catch(err => {
        console.error('Failed to send message:', err);
    });
}

function scrollToBottom() {
    const container = document.getElementById('messages-container');
    container.scrollTop = container.scrollHeight;
}

