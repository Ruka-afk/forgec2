export function esc(s: string): string {
  return s.replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;")
    .replace(/\//g, "&#x2F;");
}

export function sanitizeHtml(html: string): string {
  html = html.replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, "");
  html = html.replace(/<iframe\b[^>]*\/?>/gi, "");
  html = html.replace(/<embed\b[^>]*\/?>/gi, "");
  html = html.replace(/<object\b[^>]*>.*?<\/object>/gi, "");
  html = html.replace(/<svg\b[^>]*onload\s*=[^>]*\/?>/gi, "");
  html = html.replace(/\s+on\w+(?:\s*=\s*(?:"[^"]*"|'[^']*'|`[^`]*`|[^\s>]*))?/gi, "");
  html = html.replace(/(?:href|src|action|formaction)\s*=\s*(?:"[^"]*"|'[^']*'|`[^`]*`|[^\s>]*)/gi, (match) => {
    if (/javascript|data:/i.test(match)) return match.replace(/=\s*["'`].*["'`]/, '="#"');
    return match;
  });
  return html;
}
