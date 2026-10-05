import { esc, escAttr } from "./sanitize";

function safeMarkdownHref(escapedValue: string): string | null {
  // `inline` escapes the complete source before parsing Markdown, so the
  // captured href holds HTML entities where the source had &, <, >, ", ' or
  // /. Decode everything the escaper can emit before gating (ampersand last,
  // so a double-encoded &amp;#x2F; cannot decode twice), then re-escape for
  // the attribute. escAttr (not esc) keeps "/" unescaped so the href stays a
  // canonical URL; & escaping still breaks entity-based scheme obfuscation.
  const href = escapedValue.trim()
    .replace(/&#x2F;/g, "/")
    .replace(/&#39;/g, "'")
    .replace(/&quot;/g, '"')
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&amp;/g, "&");
  return /^(?:https?:\/\/|mailto:)/i.test(href) ? escAttr(href) : null;
}

function inline(text: string): string {
  let t = esc(text);
  t = t.replace(/`([^`]+)`/g, (_m, c) => `<code>${c}</code>`);
  t = t.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  t = t.replace(/__([^_]+)__/g, "<strong>$1</strong>");
  t = t.replace(/\*([^*]+)\*/g, "<em>$1</em>");
  t = t.replace(/_([^_]+)_/g, "<em>$1</em>");
  t = t.replace(/\[(source|来源)\s*:\s*([^\]\r\n]{1,180})\]/gi, '<span class="ai-source-ref">$1: $2</span>');
  t = t.replace(/\[([^\]]+)]\(([^)\s]+)\)/g, (match, label: string, href: string) => {
    const safeHref = safeMarkdownHref(href);
    return safeHref ? `<a href="${safeHref}" target="_blank" rel="noopener noreferrer">${label}</a>` : match;
  });
  return t;
}

function splitTableRow(line: string): string[] {
  return line.trim().replace(/^\|/, "").replace(/\|$/, "").split("|").map((cell) => cell.trim());
}

function isTableDivider(line: string): boolean {
  const cells = splitTableRow(line);
  return cells.length > 0 && cells.every((cell) => /^:?-{3,}:?$/.test(cell));
}

// renderMarkdown converts a limited, safe subset of Markdown to sanitized HTML.
// It escapes all raw input first, so user/agent content cannot inject markup.
// This is a synchronous version for use in dangerouslySetInnerHTML.
export function renderMarkdown(src: string): string {
  if (!src) return "";

  const lines = src.replace(/\r\n/g, "\n").split("\n");
  const blocks: string[] = [];
  let i = 0;
  // Each pass must consume at least one line. A missed increment used to
  // spin forever on blank lines — which every restored chat transcript has.
  const maxSteps = lines.length + 2;
  let steps = 0;

  while (i < lines.length) {
    if (++steps > maxSteps) break;
    const line = lines[i];

    if (line.trim() === "") {
      i++;
      continue;
    }

    // Fenced code block
    if (/^```/.test(line)) {
      const lang = line.slice(3).trim().replace(/[^a-z0-9_+#.-]/gi, "").slice(0, 32);
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !/^```/.test(lines[i])) {
        codeLines.push(esc(lines[i]));
        i++;
      }
      i++; // skip closing ```
      const cls = lang ? ` class="language-${esc(lang)}"` : "";
      const label = lang ? `<div class="md-code-head"><span>${esc(lang.toUpperCase())}</span></div>` : "";
      blocks.push(`<div class="md-code-block">${label}<pre><code${cls}>${codeLines.join("\n")}</code></pre></div>`);
      continue;
    }

    // Heading
    const headingMatch = line.match(/^(#{1,6})\s+(.*)/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      blocks.push(`<h${level}>${inline(headingMatch[2])}</h${level}>`);
      i++;
      continue;
    }

    // Blockquote
    if (/^>\s?/.test(line)) {
      const quoteLines: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) {
        quoteLines.push(lines[i].replace(/^>\s?/, ""));
        i++;
      }
      blocks.push(`<blockquote>${inline(quoteLines.join("\n"))}</blockquote>`);
      continue;
    }

    // Horizontal rule
    if (/^(---|\*\*\*|___)\s*$/.test(line)) {
      blocks.push("<hr/>");
      i++;
      continue;
    }

    // Compact GitHub-style table. Rendering is intentionally limited to
    // plain cells plus the safe inline subset above.
    if (line.includes("|") && i + 1 < lines.length && isTableDivider(lines[i + 1])) {
      const headers = splitTableRow(line);
      i += 2;
      const rows: string[][] = [];
      while (i < lines.length && lines[i].trim() !== "" && lines[i].includes("|")) {
        rows.push(splitTableRow(lines[i]));
        i++;
      }
      blocks.push(`<div class="md-table-wrap"><table><thead><tr>${headers.map((cell) => `<th>${inline(cell)}</th>`).join("")}</tr></thead><tbody>${rows.map((row) => `<tr>${headers.map((_, index) => `<td>${inline(row[index] || "")}</td>`).join("")}</tr>`).join("")}</tbody></table></div>`);
      continue;
    }

    // Unordered list
    if (/^[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^[-*]\s+/.test(lines[i])) {
        items.push(`<li>${inline(lines[i].replace(/^[-*]\s+/, ""))}</li>`);
        i++;
      }
      blocks.push(`<ul>${items.join("")}</ul>`);
      continue;
    }

    // Ordered list
    if (/^\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s+/.test(lines[i])) {
        items.push(`<li>${inline(lines[i].replace(/^\d+\.\s+/, ""))}</li>`);
        i++;
      }
      blocks.push(`<ol>${items.join("")}</ol>`);
      continue;
    }

    // Paragraph
    const para: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !/^```/.test(lines[i]) &&
      !/^(#{1,6})\s/.test(lines[i]) &&
      !/^>\s?/.test(lines[i]) &&
      !/^[-*]\s+/.test(lines[i]) &&
      !/^\d+\.\s+/.test(lines[i]) &&
      !/^(---|\*\*\*|___)\s*$/.test(lines[i]) &&
      !(lines[i].includes("|") && i + 1 < lines.length && isTableDivider(lines[i + 1]))
    ) {
      para.push(lines[i]);
      i++;
    }
    if (para.length > 0) {
      blocks.push(`<p>${inline(para.join("\n")).replace(/\n/g, "<br/>")}</p>`);
    } else {
      i++;
    }
  }

  const raw = blocks.join("\n");
  return raw;
}
