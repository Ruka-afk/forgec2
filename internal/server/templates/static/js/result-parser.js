// Smart Result Parser Module
const ResultParser = (function() {
    
    // IP address regex (IPv4)
    const IP_REGEX = /\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b/g;
    
    // URL regex
    const URL_REGEX = /\b(https?:\/\/[^\s<>"']+)/gi;
    
    // Email regex
    const EMAIL_REGEX = /\b([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})\b/g;
    
    // File path regex (Windows & Linux)
    const PATH_REGEX = /\b([A-Z]:\\(?:[^\\/:*?"<>|\r\n]+\\)*[^\\/:*?"<>|\r\n]*|\/(?:[^\/\0]+\/)*[^\/\0]+)\b/g;
    
    // Credential pattern (user:pass or user=password)
    const CRED_REGEX = /\b([a-zA-Z0-9_\\-]+)[:=]([^\s:,"']{3,})\b/g;
    
    // Parse and highlight text
    function parse(text) {
        if (!text) return '';
        
        let html = escapeHtml(text);
        
        // Highlight IPs
        html = html.replace(IP_REGEX, (match) => {
            return `<a href="https://ipinfo.io/${match}" target="_blank" class="text-blue-400 hover:text-blue-300 underline" title="查看 IP 信息">${match}</a>`;
        });
        
        // Highlight URLs
        html = html.replace(URL_REGEX, (match) => {
            return `<a href="${match}" target="_blank" class="text-cyan-400 hover:text-cyan-300 underline">${match}</a>`;
        });
        
        // Highlight emails
        html = html.replace(EMAIL_REGEX, (match) => {
            return `<a href="mailto:${match}" class="text-purple-400 hover:text-purple-300 underline">${match}</a>`;
        });
        
        // Highlight paths
        html = html.replace(PATH_REGEX, (match) => {
            return `<span class="text-yellow-400 font-mono" title="文件路径">${match}</span>`;
        });
        
        // Highlight credentials (be careful not to highlight normal colons)
        html = html.replace(CRED_REGEX, (match, user, pass) => {
            // Only highlight if it looks like credentials
            if (pass.length >= 3 && !pass.includes('/') && !pass.includes('\\')) {
                return `<span class="text-red-400 font-semibold" title="可能的凭据">${user}</span>:<span class="text-green-400 font-semibold">${pass}</span>`;
            }
            return match;
        });
        
        return html;
    }
    
    // Try to detect and convert table format
    function parseTable(text) {
        if (!text) return null;
        
        const lines = text.trim().split('\n');
        if (lines.length < 2) return null;
        
        // Try to detect delimiter
        const firstLine = lines[0];
        let delimiter = null;
        
        if (firstLine.includes('\t')) {
            delimiter = '\t';
        } else if (firstLine.includes(',')) {
            delimiter = ',';
        } else if (firstLine.match(/\s{2,}/)) {
            delimiter = /\s{2,}/; // Multiple spaces
        }
        
        if (!delimiter) return null;
        
        // Parse rows
        const rows = lines.map(line => {
            if (delimiter instanceof RegExp) {
                return line.split(delimiter).map(cell => cell.trim());
            } else {
                return line.split(delimiter).map(cell => cell.trim());
            }
        });
        
        // Check if all rows have same number of columns
        const colCount = rows[0].length;
        if (colCount < 2 || colCount > 10) return null;
        if (!rows.every(row => row.length === colCount)) return null;
        
        // Build HTML table
        let html = '<table class="w-full border-collapse border border-slate-600 my-2">';
        html += '<thead class="bg-slate-700">';
        html += '<tr>';
        rows[0].forEach(cell => {
            html += `<th class="border border-slate-600 px-3 py-2 text-left text-xs font-semibold text-slate-300">${escapeHtml(cell)}</th>`;
        });
        html += '</tr></thead>';
        
        html += '<tbody>';
        for (let i = 1; i < rows.length; i++) {
            html += '<tr class="hover:bg-slate-700/50">';
            rows[i].forEach(cell => {
                html += `<td class="border border-slate-600 px-3 py-2 text-sm text-slate-300">${parse(cell)}</td>`;
            });
            html += '</tr>';
        }
        html += '</tbody></table>';
        
        return html;
    }
    
    // Try to format as JSON
    function formatJSON(text) {
        try {
            const obj = JSON.parse(text);
            const formatted = JSON.stringify(obj, null, 2);
            return `<pre class="bg-slate-900 p-3 rounded-lg overflow-x-auto text-xs text-green-400 font-mono">${escapeHtml(formatted)}</pre>`;
        } catch (e) {
            return null;
        }
    }
    
    // Detect error patterns
    function highlightErrors(text) {
        let html = text;
        
        // Error keywords
        const errorPatterns = [
            /\b(error|err|fail|failed|failure|exception|denied|invalid|unauthorized|timeout)\b/gi,
            /\b(错误 | 失败 | 拒绝 | 无效 | 未授权 | 超时)\b/g
        ];
        
        errorPatterns.forEach(pattern => {
            html = html.replace(pattern, (match) => {
                return `<span class="text-red-400 font-bold">${match}</span>`;
            });
        });
        
        // Success keywords
        const successPatterns = [
            /\b(success|successful|ok|done|completed)\b/gi,
            /\b(成功 | 完成 | 已完成)\b/g
        ];
        
        successPatterns.forEach(pattern => {
            html = html.replace(pattern, (match) => {
                return `<span class="text-green-400 font-bold">${match}</span>`;
            });
        });
        
        return html;
    }
    
    // Smart parse with all methods
    function smartParse(text) {
        if (!text) return '<span class="text-slate-500 italic">空结果</span>';
        
        // Try JSON first
        const jsonHtml = formatJSON(text);
        if (jsonHtml) return jsonHtml;
        
        // Try table
        const tableHtml = parseTable(text);
        if (tableHtml) return tableHtml;
        
        // Regular text with highlights
        let html = parse(text);
        html = highlightErrors(html);
        
        return `<pre class="whitespace-pre-wrap break-words">${html}</pre>`;
    }
    
    // Extract IPs from text
    function extractIPs(text) {
        const matches = text.match(IP_REGEX);
        return matches ? [...new Set(matches)] : [];
    }
    
    // Extract URLs from text
    function extractURLs(text) {
        const matches = text.match(URL_REGEX);
        return matches ? [...new Set(matches)] : [];
    }
    
    // Extract credentials from text
    function extractCredentials(text) {
        const creds = [];
        let match;
        while ((match = CRED_REGEX.exec(text)) !== null) {
            if (match[2].length >= 3) {
                creds.push({ user: match[1], pass: match[2] });
            }
        }
        return creds;
    }
    
    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
    
    return {
        parse,
        parseTable,
        formatJSON,
        highlightErrors,
        smartParse,
        extractIPs,
        extractURLs,
        extractCredentials
    };
})();

// Export to global scope
window.ResultParser = ResultParser;
