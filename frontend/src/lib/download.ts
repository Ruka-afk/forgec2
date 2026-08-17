export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export function downloadText(content: string, filename: string, mime = "text/plain") {
  downloadBlob(new Blob([content], { type: mime }), filename);
}

export function downloadJSON(data: unknown, filename: string) {
  downloadText(JSON.stringify(data, null, 2), filename, "application/json");
}

export function downloadBase64(b64: string, filename: string, mime = "application/octet-stream") {
  const bytes = Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
  downloadBlob(new Blob([bytes], { type: mime }), filename);
}

function parseFilename(header: string | null, fallback: string): string {
  if (!header) return fallback;
  const m = header.match(/filename=([^;]+)/);
  return m ? m[1].trim().replace(/^"|"$/g, "") : fallback;
}

export function downloadFromResponse(res: Response, fallback: string) {
  const filename = parseFilename(res.headers.get("Content-Disposition"), fallback);
  return res.blob().then((blob) => downloadBlob(blob, filename));
}
