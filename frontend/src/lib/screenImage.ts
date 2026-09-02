"use client";

export function dataUrlToBlobUrl(dataUrl: string): string {
  if (dataUrl.startsWith("blob:")) return dataUrl;
  const comma = dataUrl.indexOf(",");
  if (comma === -1) return dataUrl;
  const meta = dataUrl.slice(0, comma);
  const b64 = dataUrl.slice(comma + 1);
  const mime = meta.match(/data:([^;]+)/)?.[1] || "image/png";
  try {
    const bin = atob(b64);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    return URL.createObjectURL(new Blob([bytes], { type: mime }));
  } catch {
    return dataUrl;
  }
}

export function revokeBlobUrl(url: string) {
  if (url.startsWith("blob:")) {
    try { URL.revokeObjectURL(url); } catch { void 0; }
  }
}
