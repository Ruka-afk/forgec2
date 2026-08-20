import { useMemo } from "react";

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

const SENSITIVE_PATTERNS: Array<{ re: RegExp; cls: "secret" | "email" | "url" | "ip" | "user" }> = [
  // password/secret/token/api key assignments — value is highlighted
  { re: /\b(pass(?:word)?|passwd|pwd|secret|apikey|api_key|token|key|pin)\s*[=:]\s*([^\s;,&|]+)/gi, cls: "secret" },
  // email addresses
  { re: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b/g, cls: "email" },
  // URLs
  { re: /\bhttps?:\/\/[^\s<>"']+/g, cls: "url" },
  // IPv4/IPv6 addresses
  { re: /\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b/g, cls: "ip" },
  // username=value and user@domain pairs
  { re: /\b(username|user|login|account)\s*[=:]\s*([^\s;,&|]+)/gi, cls: "user" },
];

function highlightKeylog(text: string): string {
  const result = escapeHtml(text);
  const used: Array<[number, number, string]> = [];
  for (const { re, cls } of SENSITIVE_PATTERNS) {
    for (const m of text.matchAll(re)) {
      const idx = m.index ?? 0;
      const len = m[0].length;
      if (used.some(([s, e]) => idx < e && idx + len > s)) continue;
      used.push([idx, idx + len, cls]);
    }
  }
  if (used.length === 0) return result;
  used.sort((a, b) => a[0] - b[0]);
  let out = "";
  let pos = 0;
  for (const [s, e, cls] of used) {
    out += result.slice(pos, s);
    out += `<mark class="keylog-${cls}">${result.slice(s, e)}</mark>`;
    pos = e;
  }
  out += result.slice(pos);
  return out;
}

export default function KeylogContent({ text, className = "" }: { text: string; className?: string }) {
  const html = useMemo(() => highlightKeylog(text || ""), [text]);
  return <div className={className} dangerouslySetInnerHTML={{ __html: html }} />;
}