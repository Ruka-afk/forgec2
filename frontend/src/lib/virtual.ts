export interface VirtualRange {
  start: number;
  end: number;
  offsetTop: number;
  totalHeight: number;
  visibleCount: number;
}

/**
 * Compute a window of items to render for fixed-height virtual lists.
 * Pure helper — no React. Overscan adds extra rows above/below the viewport.
 */
export function computeVirtualRange(
  count: number,
  rowHeight: number,
  scrollTop: number,
  viewportHeight: number,
  overscan = 6,
): VirtualRange {
  const safeCount = Math.max(0, count | 0);
  const h = Math.max(1, rowHeight | 0);
  const totalHeight = safeCount * h;
  if (safeCount === 0) {
    return { start: 0, end: 0, offsetTop: 0, totalHeight: 0, visibleCount: 0 };
  }
  const visible = Math.ceil(Math.max(0, viewportHeight) / h) + overscan * 2;
  const maxStart = Math.max(0, safeCount - Math.max(1, visible));
  const rawStart = Math.floor(Math.max(0, scrollTop) / h) - overscan;
  const start = Math.min(maxStart, Math.max(0, rawStart));
  const end = Math.min(safeCount, start + Math.max(1, visible));
  return {
    start,
    end,
    offsetTop: start * h,
    totalHeight,
    visibleCount: end - start,
  };
}

/** Prefer full render under this size — virtualization overhead not worth it. */
export const VIRTUAL_THRESHOLD = 30;
