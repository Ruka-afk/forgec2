"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/**
 * Keyed "copied" feedback for copy buttons: `copy(text, key)` writes to the
 * clipboard, marks `key` as copied, and reverts to "" after `revertMs`.
 * The timer is cleaned up on unmount (no setState after unmount leaks).
 */
export function useCopiedField(revertMs = 1500): {
  copiedField: string;
  copy: (text: string, key: string) => void;
} {
  const [copiedField, setCopiedField] = useState("");
  const timerRef = useRef<number | null>(null);

  const copy = useCallback(
    (text: string, key: string) => {
      navigator.clipboard
        .writeText(text)
        .then(() => {
          if (timerRef.current !== null) window.clearTimeout(timerRef.current);
          setCopiedField(key);
          timerRef.current = window.setTimeout(() => {
            timerRef.current = null;
            setCopiedField("");
          }, revertMs);
        })
        .catch(() => {});
    },
    [revertMs]
  );

  useEffect(
    () => () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    },
    []
  );

  return { copiedField, copy };
}