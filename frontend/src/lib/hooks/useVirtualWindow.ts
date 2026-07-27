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

  const virtualized = enabled && count >= threshold;

  const measure = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setViewportHeight(el.clientHeight || 480);
    setScrollTop(el.scrollTop);
  }, []);

  const onScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setScrollTop(el.scrollTop);
  }, []);

  useEffect(() => {
    if (!virtualized) return;
    measure();
    const el = scrollRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(() => measure());
    ro.observe(el);
    return () => ro.disconnect();
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
