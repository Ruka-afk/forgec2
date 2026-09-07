import { isSafeImageSrc } from "@/lib/safeUrl";

export function screenshotDataUrl(data: unknown): string {
  if (!data || typeof data !== "object") return "";
  const rec = data as Record<string, unknown>;
  const inner = rec.data && typeof rec.data === "object" && !Array.isArray(rec.data)
    ? rec.data as Record<string, unknown>
    : rec;
  const img = String(inner.image ?? rec.image ?? "");
  // Reject non-image and SVG data URLs (SVG can carry scripts even in <img>).
  return isSafeImageSrc(img) ? img : "";
}

export function shouldRefreshDockShot(msg: Record<string, unknown>): boolean {
  return String(msg.task_type || "") === "screenshot" && String(msg.status || "") === "completed";
}
