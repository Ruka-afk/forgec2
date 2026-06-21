const STORAGE_KEY = 'forgec2_ai_messages';
const MAX_CONTEXT = 20; // max user+assistant pairs
let messages = [];
let isGenerating = false;
let abortController = null;
let _lastRenderTime = 0;

// Trim oldest messages to stay within context window
function trimContext() {
    while (messages.length > MAX_CONTEXT * 2) {
        messages.shift(); // remove oldest user message
        if (messages[0] && messages[0].role === 'assistant') messages.shift(); // remove its response too
    }
    updateMessageCount();
}

function updateMessageCount() {
    const el = document.getElementById('msg-count');
    if (el) el.textContent = `${messages.length}/${MAX_CONTEXT * 2}`;
}

// Load saved messages on startup
(function loadMessages() {
    function restore() {
        try {
            const saved = localStorage.getItem(STORAGE_KEY);
            if (saved) {
                messages = JSON.parse(saved);
                const welcome = document.getElementById('ai-welcome');
                if (welcome) welcome.classList.add('hidden');

                messages.forEach(m => {
                    if (m.role === 'user') appendMessage('user', m.content);
                    else if (m.role === 'assistant') {
                        const div = appendMessage('assistant', m.content);
                        const contentEl = div.querySelector('.ai-content');
                        if (contentEl) contentEl.innerHTML = renderMarkdown(m.content);
                        if (m.reasoning) {
                            const reasoningDiv = document.createElement('div');
                            reasoningDiv.className = 'ai-reasoning bg-amber-50 border border-amber-200 rounded-xl px-3 py-2 mt-2 text-xs text-amber-800';
                            reasoningDiv.innerHTML = '<details open><summary class="cursor-pointer font-medium">思考过程 <i class="fa-solid fa-chevron-down ml-1 text-[10px]"></i></summary><div class="mt-1 whitespace-pre-wrap">' + escapeHtml(m.reasoning) + '</div></details>';
                            div.querySelector('.bg-white').appendChild(reasoningDiv);
                        }
                    }
                });
                scrollToBottom();
                trimContext();
                updateMessageCount();
                setTimeout(() => document.getElementById('ai-input').focus(), 200);
            }
        } catch(e) { messages = []; }
    }
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', restore);
    } else {
        restore();
    }
})();

function saveMessages() {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(messages)); } catch(e) {}
}

function clearChat() {
    if (!confirm('清除所有聊天记录？')) return;
    messages = [];
    localStorage.removeItem(STORAGE_KEY);
    document.getElementById('ai-messages').querySelectorAll('.flex.gap-3').forEach(el => el.remove());
    document.getElementById('ai-welcome').classList.remove('hidden');
    updateMessageCount();
}

function toggleSettings() {
    document.getElementById('ai-settings').classList.toggle('hidden');
}

function onProviderChange() {
    const provider = document.getElementById('cfg-provider').value;
    const epGroup = document.getElementById('cfg-endpoint-group');
    epGroup.classList.toggle('hidden', provider !== 'custom');

    const modelMap = { deepseek: 'deepseek-chat', openai: 'gpt-4o', claude: 'claude-3-5-sonnet-20241022', qianwen: 'qwen-plus', custom: '' };
    document.getElementById('cfg-model').value = modelMap[provider] || '';
}

const promptTemplates = {
    redteam: `你是 ForgeC2 红队行动助手，运行在 C2 服务器上。你可以列出在线 Implant、查看目标详情、执行命令、查看凭据、管理监听器等。用中文回复。`,
    concise: `你是安全分析助手。回答要简洁准确，用要点列表。避免长篇解释。优先给出可执行的操作步骤。`,
    verbose: `你是资深渗透测试专家。回答要详尽专业，包含技术细节。使用 MITRE ATT&CK 术语。输出结构化的操作报告。`,
    social: `你是社会工程学专家。帮助设计钓鱼邮件、社工话术。输出格式：场景设定→话术脚本→注意事项。用中文回复。`
};

function onPromptTemplateChange() {
    const sel = document.getElementById('prompt-template');
    const ta = document.querySelector('[name="system_prompt"]');
    if (sel.value && promptTemplates[sel.value]) ta.value = promptTemplates[sel.value];
}

async function saveAIConfig(e) {
    e.preventDefault();
    const form = document.getElementById('ai-config-form');
    const data = {
        enabled: form.querySelector('[name="enabled"]').checked,
        provider: form.querySelector('[name="provider"]').value,
        api_key: form.querySelector('[name="api_key"]').value,
        model: form.querySelector('[name="model"]').value,
        endpoint: form.querySelector('[name="endpoint"]').value,
        system_prompt: form.querySelector('[name="system_prompt"]').value
    };
    try {
        const resp = await fetch('/ai/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
            body: JSON.stringify(data)
        });
        const result = await resp.json();
        const el = document.getElementById('ai-config-result');
        el.innerHTML = result.success
            ? '<div class="text-emerald-600 text-xs mt-1">配置已保存，刷新页面生效</div>'
            : '<div class="text-red-600 text-xs mt-1">' + escapeHtml(result.error || 'Error') + '</div>';
        if (result.success) setTimeout(() => location.reload(), 1500);
    } catch (err) {
        showToast('Save failed: ' + err.message, 'error');
    }
}

function sendQuick(text) {
    document.getElementById('ai-input').value = text;
    sendMessage();
}

function stopGeneration() {
    if (abortController) {
        abortController.abort();
        abortController = null;
    }
}

function retryLast() {
    if (messages.length === 0) return;
    const last = messages[messages.length - 1];
    if (last.role !== 'user') return;
    messages.pop();
    saveMessages();
    const msgs = document.getElementById('ai-messages');
    const children = msgs.querySelectorAll('.flex.gap-3');
    if (children.length >= 2) {
        children[children.length - 1].remove();
        children[children.length - 2].remove();
    }
    document.getElementById('ai-input').value = last.content;
    document.getElementById('ai-input').focus();
    sendMessage();
}

function scrollToBottomFloat() {
    const container = document.getElementById('ai-messages');
    const btn = document.getElementById('scroll-bottom-btn');
    if (!container || !btn) return;
    const dist = container.scrollHeight - container.scrollTop - container.clientHeight;
    btn.classList.toggle('hidden', dist < 200);
}

function exportChat() {
    let md = '# ForgeC2 AI 对话记录\n\n';
    messages.forEach(m => {
        if (m.role === 'user') md += `**You:** ${m.content}\n\n`;
        else {
            md += `**AI:** ${m.content}\n\n`;
            if (m.reasoning) md += `<details><summary>思考过程</summary>\n\n${m.reasoning}\n\n</details>\n\n`;
        }
    });
    const blob = new Blob([md], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = `forgec2-chat-${new Date().toISOString().slice(0,10)}.md`;
    a.click(); URL.revokeObjectURL(url);
    showToast('对话已导出', 'success');
}

function resetSendButton() {
    const btn = document.getElementById('ai-send-btn');
    btn.innerHTML = '<i class="fa-solid fa-paper-plane text-sm"></i>';
    btn.classList.add('bg-indigo-600', 'hover:bg-indigo-700');
    btn.classList.remove('bg-red-500', 'hover:bg-red-600');
}

async function sendMessage() {
    if (isGenerating) {
        stopGeneration();
        return;
    }
    const input = document.getElementById('ai-input');
    const text = input.value.trim();
    if (!text) return;
    input.value = '';
    input.style.height = 'auto';
    input.focus();
    isGenerating = true; // set BEFORE append to prevent re-entry

    appendMessage('user', text);
    messages.push({ role: 'user', content: text });
    trimContext(); // keep context window manageable
    saveMessages();

    const msgDiv = appendMessage('assistant', '');
    const contentDiv = msgDiv.querySelector('.ai-content');

    // Add thinking indicator
    const thinkingDiv = document.createElement('div');
    thinkingDiv.className = 'ai-thinking flex items-center gap-2 text-slate-400 text-xs mt-1';
    thinkingDiv.innerHTML = '<i class="fa-solid fa-spinner fa-spin mr-1"></i><span>AI 正在思考...</span>';
    msgDiv.querySelector('.bg-white').appendChild(thinkingDiv);

    // Add reasoning box (hidden initially)
    const reasoningDiv = document.createElement('div');
    reasoningDiv.className = 'ai-reasoning hidden bg-amber-50 border border-amber-200 rounded-xl px-3 py-2 mt-2 text-xs text-amber-800';
    reasoningDiv.innerHTML = '<details><summary class="cursor-pointer font-medium">思考过程 <i class="fa-solid fa-chevron-down ml-1 text-[10px]"></i></summary><div class="mt-1 whitespace-pre-wrap"></div></details>';
    msgDiv.querySelector('.bg-white').appendChild(reasoningDiv);

    let reasoningContent = '';

    // Helper functions (closure to access reasoningContent)
    function showThinking(show) {
        thinkingDiv.style.display = show ? '' : 'none';
    }
    function showReasoning(text) {
        reasoningContent += text;
        reasoningDiv.classList.remove('hidden');
        const detailDiv = reasoningDiv.querySelector('div');
        if (detailDiv) detailDiv.textContent = reasoningContent;
        scrollToBottom();
    }

    document.getElementById('ai-send-btn').innerHTML = '<i class="fa-solid fa-stop text-sm"></i>';
    document.getElementById('ai-send-btn').classList.add('bg-red-500', 'hover:bg-red-600');
    document.getElementById('ai-send-btn').classList.remove('bg-indigo-600', 'hover:bg-indigo-700');

    abortController = new AbortController();

    try {
        const resp = await fetch('/ai/chat', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
            body: JSON.stringify({ messages: messages }),
            signal: abortController.signal
        });

        if (!resp.ok) {
            let errMsg = 'HTTP ' + resp.status;
            try { const err = await resp.json(); errMsg = err.error || errMsg; } catch(e) {}
            contentDiv.innerHTML = `<div class="text-red-500 text-sm">${escapeHtml(errMsg)}</div>
                <button onclick="retryLast()" class="mt-2 text-xs px-3 py-1 bg-red-50 hover:bg-red-100 text-red-600 rounded-lg">🔄 重试</button>`;
            showThinking(false);
            resetSendButton();
            isGenerating = false;
            return;
        }

        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let fullContent = '';

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });

            // Process complete SSE events (delimited by \n\n or \r\n\r\n)
            const delim = buffer.includes('\r\n\r\n') ? '\r\n\r\n' : '\n\n';
            while (buffer.includes(delim)) {
                const idx = buffer.indexOf(delim);
                const raw = buffer.slice(0, idx).replace(/\r/g, '');
                buffer = buffer.slice(idx + delim.length);

                const evt = {};
                for (const line of raw.split('\n')) {
                    if (line.startsWith('event: ')) evt.event = line.slice(7);
                    else if (line.startsWith('data: ')) evt.data = (evt.data ? evt.data + '\n' : '') + line.slice(6);
                }
                if (!evt.event) continue;
                if (evt.event !== 'thinking' && evt.event !== 'clear' && !evt.data) continue;

                if (evt.event === 'thinking') {
                    showThinking(true);
                } else if (evt.event === 'clear') {
                    showThinking(false);
                } else if (evt.event === 'text') {
                    fullContent = evt.data;
                    if (Date.now() - _lastRenderTime > 50) {
                        contentDiv.innerHTML = renderMarkdown(fullContent);
                        _lastRenderTime = Date.now();
                    }
                    scrollToBottom();
                } else if (evt.event === 'reasoning') {
                    showReasoning(evt.data);
                } else if (evt.event === 'tool') {
                    try {
                        const td = JSON.parse(evt.data);
                        showToolCall(td.name, td.result);
                    } catch(e) {}
                } else if (evt.event === 'error') {
                    contentDiv.innerHTML += '<span class="text-red-500">' + escapeHtml(evt.data) + '</span>';
                }
            }
        }

        // Final render to ensure complete display (bypass throttle)
        if (fullContent) {
            contentDiv.innerHTML = renderMarkdown(fullContent);
            messages.push({ role: 'assistant', content: fullContent, reasoning: reasoningContent });
            saveMessages();
        }
    } catch (err) {
        if (err.name === 'AbortError') {
            if (fullContent) {
                fullContent += '\n\n*[已停止]*';
                contentDiv.innerHTML = renderMarkdown(fullContent);
                messages.push({ role: 'assistant', content: fullContent, reasoning: reasoningContent });
                saveMessages();
            }
        } else {
            contentDiv.innerHTML = `<div class="text-red-500 text-sm">${escapeHtml(err.message)}</div>
                <button onclick="retryLast()" class="mt-2 text-xs px-3 py-1 bg-red-50 hover:bg-red-100 text-red-600 rounded-lg">🔄 重试</button>`;
        }
    } finally {
        isGenerating = false;
        abortController = null;
        showThinking(false);
        resetSendButton();
        scrollToBottom();
    }
}

function appendMessage(role, content) {
    const container = document.getElementById('ai-messages');
    const div = document.createElement('div');
    div.className = 'flex gap-3' + (role === 'user' ? ' flex-row-reverse' : '');

    if (role === 'user') {
        div.innerHTML = `<div class="w-8 h-8 bg-slate-200 rounded-xl flex items-center justify-center flex-shrink-0 mt-1">
            <i class="fa-solid fa-user text-slate-500 text-sm"></i></div>
            <div class="bg-indigo-50 border border-indigo-200 rounded-2xl px-4 py-3 max-w-[85%]">
                <p class="text-sm text-slate-800 whitespace-pre-wrap">${escapeHtml(content)}</p>
            </div>`;
    } else {
        div.innerHTML = `<div class="w-8 h-8 bg-indigo-100 rounded-xl flex items-center justify-center flex-shrink-0 mt-1">
            <i class="fa-solid fa-robot text-indigo-500 text-sm"></i></div>
            <div class="bg-white border border-slate-200 rounded-2xl px-4 py-3 shadow-sm max-w-[85%] relative group">
                <div class="ai-content text-sm text-slate-700 prose prose-sm max-w-none"></div>
                <button class="copy-msg absolute top-2 right-2 p-1.5 rounded-lg bg-white/80 hover:bg-slate-100 text-slate-400 hover:text-indigo-600 opacity-0 group-hover:opacity-100 transition-opacity text-xs" title="复制" onclick="copyMessage(this)"><i class="fa-solid fa-copy"></i></button>
                <button class="copy-msg absolute top-2 right-10 p-1.5 rounded-lg bg-white/80 hover:bg-slate-100 text-slate-400 hover:text-indigo-600 opacity-0 group-hover:opacity-100 transition-opacity text-xs" title="重新生成" onclick="retryLast()"><i class="fa-solid fa-rotate-right"></i></button>
            </div>`;
    }

    container.appendChild(div);
    scrollToBottom();
    return div;
}

function showToolCall(name, resultJSON) {
    const container = document.getElementById('ai-tool-calls');
    const div = document.createElement('div');
    div.className = 'bg-amber-50 border border-amber-200 rounded-xl px-4 py-3 text-xs';

    let resultPreview = '';
    try {
        const r = JSON.parse(resultJSON);
        if (typeof r === 'string') {
            resultPreview = r.substring(0, 300);
        } else if (Array.isArray(r)) {
            resultPreview = JSON.stringify(r.slice(0, 5), null, 1).substring(0, 500);
        } else {
            resultPreview = JSON.stringify(r, null, 1).substring(0, 500);
        }
    } catch(e) {
        resultPreview = resultJSON.substring(0, 300);
    }

    div.innerHTML = `<div class="flex items-center gap-2 mb-1">
        <i class="fa-solid fa-wrench text-amber-500"></i>
        <span class="font-semibold text-amber-800">${escapeHtml(name)}</span>
        <span class="text-slate-400">调用工具</span></div>
        <pre class="text-amber-700 mt-1 whitespace-pre-wrap font-mono text-[11px] leading-relaxed">${escapeHtml(resultPreview)}</pre>`;
    container.appendChild(div);
    setTimeout(() => div.style.opacity = '0' && setTimeout(() => div.remove(), 500), 15000);
    scrollToBottom();
}

function renderMarkdown(text) {
    let html = escapeHtml(text);

    // Extract and protect code blocks before any other processing
    const codeBlocks = [];
    html = html.replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) => {
        codeBlocks.push({ lang, code });
        return '%%CODEBLOCK_' + (codeBlocks.length - 1) + '%%';
    });

    // Inline code
    html = html.replace(/`([^`]+)`/g, '<code class="bg-slate-100 text-indigo-600 px-1.5 py-0.5 rounded text-xs font-mono">$1</code>');

    // Tables: | col1 | col2 |\n|---|---|\n| v1 | v2 |
    html = html.replace(/((?:^\|.+\|\n?)+)/gm, (match) => {
        const lines = match.trim().split('\n');
        if (lines.length < 2) return match;
        // Must have header row and separator row
        const hasSep = lines[1] && /^\|[\s\-:|]+\|$/.test(lines[1]);
        if (!hasSep) return match;
        let table = '<table class="w-full text-xs border-collapse my-2"><thead><tr>';
        const headers = lines[0].split('|').filter(s => s.trim());
        headers.forEach(h => { table += '<th class="border border-slate-300 bg-slate-50 px-2 py-1 text-left font-medium">' + h.trim() + '</th>'; });
        table += '</tr></thead><tbody>';
        for (let i = 2; i < lines.length; i++) {
            const cells = lines[i].split('|').filter(s => s.trim());
            if (cells.length === 0) continue;
            table += '<tr>';
            cells.forEach(c => { table += '<td class="border border-slate-200 px-2 py-1">' + c.trim() + '</td>'; });
            table += '</tr>';
        }
        table += '</tbody></table>';
        return table;
    });

    html = html.replace(/\*\*([^*]+)\*\*/g, '<strong class="font-semibold">$1</strong>');
    html = html.replace(/\*([^*]+)\*/g, '<em>$1</em>');
    html = html.replace(/^### (.+)$/gm, '<h4 class="text-sm font-semibold text-slate-800 mt-2 mb-1">$1</h4>');
    html = html.replace(/^## (.+)$/gm, '<h3 class="text-base font-semibold text-slate-800 mt-3 mb-1">$1</h3>');
    html = html.replace(/^# (.+)$/gm, '<h2 class="text-lg font-semibold text-slate-800 mt-3 mb-2">$1</h2>');
    html = html.replace(/^- (.+)$/gm, '<li class="ml-4 list-disc">$1</li>');
    html = html.replace(/^(\d+)\. (.+)$/gm, '<li class="ml-4 list-decimal">$1. $2</li>');
    html = html.replace(/^>(.+)$/gm, '<blockquote class="border-l-2 border-indigo-300 pl-3 italic text-slate-500">$1</blockquote>');
    html = html.replace(/\n/g, '<br>');

    // Restore code blocks
    html = html.replace(/%%CODEBLOCK_(\d+)%%/g, (_, i) => {
        const cb = codeBlocks[i];
        const codeHtml = (cb.lang ? '<span class="text-slate-500 text-[10px]">' + cb.lang + '\n</span>' : '') + cb.code;
        return '<pre class="bg-slate-900 text-emerald-400 p-3 rounded-xl my-2 overflow-x-auto text-xs"><code>' + codeHtml + '</code></pre>';
    });

    return html;
}

function scrollToBottom() {
    const container = document.getElementById('ai-messages');
    setTimeout(() => { container.scrollTop = container.scrollHeight; }, 50);
}

function copyMessage(btn) {
    const contentDiv = btn.parentElement.querySelector('.ai-content');
    if (contentDiv) {
        navigator.clipboard.writeText(contentDiv.textContent).then(() => showToast('已复制', 'success'));
    }
}

function copyCode(id) {
    const el = document.getElementById(id);
    if (el) {
        navigator.clipboard.writeText(el.textContent).then(() => showToast('代码已复制', 'success'));
    }
}

// Auto-resize textarea
document.getElementById('ai-input').addEventListener('input', function() {
    this.style.height = 'auto';
    this.style.height = Math.min(this.scrollHeight, 128) + 'px';
});

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        const panel = document.getElementById('ai-settings');
        if (panel && !panel.classList.contains('hidden')) {
            panel.classList.add('hidden');
            document.getElementById('ai-input').focus();
        }
    }
});
