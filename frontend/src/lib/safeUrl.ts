// Safe URL guard for external-facing href/src values that flow from server
// or websocket data (e.g. update banner download URL, search result links,
// screenshot/loot paths, timeline event URLs). Anything that is not a
// relative path or an explicit http(s) URL is rejected, so attacker-influenced
// strings can never turn into javascript:/data:/vbscript: navigation.
export function isSafeUrl(raw: unknown): raw is string {
  if (typeof raw !== "string" || raw.length === 0 || raw.length > 4096) return false;
  if (raw.startsWith("/")) {
    // Relative path: forbid protocol-relative ("//host") and backslash
    // tricks, keep only same-origin navigation.
    if (raw.startsWith("//")) return false;
    if (raw.includes("\\")) return false;
    return true;
  }
  try {
    const u = new URL(raw);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}

// safeHref returns the value if it passes isSafeUrl, otherwise undefined so
// callers can simply write `href={safeHref(x)}` or `src={safeHref(x)}`.
export function safeHref(raw: unknown): string | undefined {
  return isSafeUrl(raw) ? raw : undefined;
}

// safeSrcForImage allows data:image/* (screenshots are embedded as data URLs)
// and blob: (via dataUrlToBlobUrl) but still blocks javascript: and friends. SVG is excluded.
export function isSafeImageSrc(raw: unknown): raw is string {
  if (typeof raw !== "string" || raw.length > 16 * 1024 * 1024) return false;
  const lower = raw.toLowerCase();
  if (lower.startsWith("blob:")) return true;
  if (!lower.startsWith("data:image/")) return isSafeUrl(raw);
  // SVG is excluded: SVG data URLs can carry embedded scripts even in <img>
  // context in some browsers.
  const afterPrefix = raw.slice(11);
  const semi = afterPrefix.indexOf(";");
  const comma = afterPrefix.indexOf(",");
  const end = semi !== -1 && (comma === -1 || semi < comma) ? semi : comma;
  const mime = (end === -1 ? afterPrefix : afterPrefix.slice(0, end)).toLowerCase();
  if (mime === "svg" || mime === "svg+xml" || mime.endsWith("/svg+xml")) return false;
  return true;
}

export function safeImageSrc(raw: unknown): string | undefined {
  return isSafeImageSrc(raw) ? raw : undefined;
}