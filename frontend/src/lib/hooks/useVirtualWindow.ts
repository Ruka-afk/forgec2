"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { computeVirtualRange, VIRTUAL_THRESHOLD, type VirtualRange } from "@/lib/virtual";

interface UseVirtualWindowOptions {
  count: number;
  rowHeight: number;
  overscan?: number;
  /** Disable windowing below threshold (default VIRTUAL_THRESHOLD). */
  threshold?: number;
  enabled?: boolean;
}

interface UseVirtualWindowResult extends VirtualRange {
  /** Attach to the scroll container. */
  scrollRef: React.RefObject<HTMLDivElement | null>;
  /** True when windowing is active (count >= threshold). */
  virtualized: boolean;
  onScroll: () => void;
}

export function useVirtualWindow({
  count,
  rowHeight,
  overscan = 6,
  threshold = VIRTUAL_THRESHOLD,
  enabled = true,
}: UseVirtualWindowOptions): UseVirtualWindowResult {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportHeight, setViewportHeight] = useState(480);
  const rafRef = useRef<number | null>(null);

  const virtualized = enabled && count >= threshold;

  const measure = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setViewportHeight(el.clientHeight || 480);
    setScrollTop(el.scrollTop);
  }, []);

  const onScroll = useCallback(() => {
    // rAF-throttle so a scroll storm triggers at most one re-render per frame,
    // and bail out entirely when windowing is inactive.
    if (!virtualized) return;
    if (rafRef.current != null) return;
    rafRef.current = requestAnimationFrame(() => {
      rafRef.current = null;
      const el = scrollRef.current;
      if (!el) return;
      setScrollTop(el.scrollTop);
    });
  }, [virtualized]);

  useEffect(() => {
    if (!virtualized) return;
    measure();
    const el = scrollRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(() => measure());
    ro.observe(el);
    return () => {
      ro.disconnect();
      if (rafRef.current != null) cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    };
  }, [virtualized, measure, count]);

  const range = virtualized
    ? computeVirtualRange(count, rowHeight, scrollTop, viewportHeight, overscan)
    : {
        start: 0,
        end: count,
        offsetTop: 0,
        totalHeight: count * rowHeight,
        visibleCount: count,
      };

  return { ...range, scrollRef, virtualized, onScroll };
}
