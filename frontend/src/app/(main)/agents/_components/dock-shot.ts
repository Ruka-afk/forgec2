export function screenshotDataUrl(data: unknown): string {
  if (!data || typeof data !== "object") return "";
  const rec = data as Record<string, unknown>;
  const inner = rec.data && typeof rec.data === "object" && !Array.isArray(rec.data)
    ? rec.data as Record<string, unknown>
    : rec;
  const img = String(inner.image ?? rec.image ?? "");
  return img.startsWith("data:image/") ? img : "";
}

export function shouldRefreshDockShot(msg: Record<string, unknown>): boolean {
  return String(msg.task_type || "") === "screenshot" && String(msg.status || "") === "completed";
}
