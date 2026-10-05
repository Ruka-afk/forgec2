import type DOMPurify from "dompurify";

let DOMPurifyPromise: Promise<typeof DOMPurify> | null = null;

function getDOMPurify(): Promise<typeof DOMPurify> {
  if (!DOMPurifyPromise) {
    DOMPurifyPromise = import("dompurify").then((m) => m.default);
  }
  return DOMPurifyPromise;
}

export function esc(s: string): string {
  return s.replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;")
    .replace(/\//g, "&#x2F;");
}

// Attribute escaper for URL-safe contexts (e.g. markdown link hrefs): escapes
// quotes/ampersands/angle brackets so the value cannot break out of the
// attribute, but leaves "/" unescaped so URLs stay canonical. Escaping slashes
// as &#x2F; only matters for JSON-in-<script> breakout, which this codebase
// does not render; entity-ified URLs would make otherwise-valid links
// non-canonical (harder to copy, inspect, and pin in tests).
export function escAttr(s: string): string {
  return s.replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

const SAFE_URI_REGEXP = /^(?:(?:(?:f|ht)tps?|mailto|tel|callto|cid|xmpp):|[^a-z]|[a-z+.-]+(?:[^a-z+.\-:]|$))/i;

export async function sanitizeHtml(html: string): Promise<string> {
  const DOMPurify = await getDOMPurify();
  const clean = DOMPurify.sanitize(html, {
    ALLOWED_TAGS: ["b", "i", "em", "strong", "a", "code", "pre", "span", "br", "ul", "ol", "li", "table", "thead", "tbody", "tr", "th", "td", "h1", "h2", "h3", "h4", "h5", "h6", "p", "div", "blockquote", "hr"],
    ALLOWED_ATTR: ["href", "target", "class", "id", "rel"],
    ALLOWED_URI_REGEXP: SAFE_URI_REGEXP,
    FORBID_ATTR: ["style", "onerror", "onload", "onclick", "onmouseover"],
  }) as string;
  // Force rel=noopener on any target=_blank that survived via allowed attrs.
  if (clean.includes('target="_blank"') && !clean.includes('rel=')) {
    return clean.replace(/target="_blank"/g, 'target="_blank" rel="noopener noreferrer"');
  }
  // Also patch existing rel-less anchors after DOMPurify if needed (string replace fallback)
  if (clean.includes('target="_blank"')) {
    return clean.replace(/<a([^>]*?)target="_blank"([^>]*?)>/g, (m, a, b) => {
      if (m.includes('rel=')) return m;
      return `<a${a}target="_blank" rel="noopener noreferrer"${b}>`;
    });
  }
  return clean;
}


