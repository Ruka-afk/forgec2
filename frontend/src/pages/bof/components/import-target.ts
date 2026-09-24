const DEFAULT_BOF_NAME = "download";

function withObjExtension(name: string): string {
  return name.toLowerCase().endsWith(".o") ? name : `${name}.o`;
}

function sanitizeBaseName(base: string): string {
  let decoded = base;
  try {
    decoded = decodeURIComponent(base);
  } catch {
    decoded = base;
  }
  return decoded.replace(/[^\w.-]/g, "_");
}

/**
 * POST /api/bof/repos/import requires BOTH `url` and `filename` and rejects a
 * request without them, so the client has to derive the object name from the
 * URL path. Query strings and fragments are dropped, the basename is sanitized,
 * and `.o` is appended when missing (the server does the same for its own
 * storage name).
 */
export function bofImportFilename(url: string, explicitName?: string): string {
  const explicit = (explicitName || "").trim();
  if (explicit) return withObjExtension(sanitizeBaseName(explicit));

  let base = "";
  try {
    const parsed = new URL(url.trim());
    base = parsed.pathname.split("/").filter(Boolean).pop() || "";
  } catch {
    base = url.trim().split(/[?#]/)[0].split("/").filter(Boolean).pop() || "";
  }
  const cleaned = sanitizeBaseName(base);
  return withObjExtension(cleaned || DEFAULT_BOF_NAME);
}
