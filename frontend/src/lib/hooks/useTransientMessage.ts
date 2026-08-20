"use client";

import { useCallback, useEffect, useRef, useState } from "react";

type TransientTone = "info" | "success" | "warning" | "destructive" | "muted";

interface TransientMessage {
  text: string;
  tone: TransientTone;
}

interface ShowOptions {
  tone?: TransientTone;
  /** Auto-dismiss delay; 0 keeps the message until dismissed or overwritten. */
  durationMs?: number;
}

/**
 * One-at-a-time transient status strip for forms and page actions
 * (render it with `<Banner tone={message.tone}>`). Each show overwrites
 * the previous message and (by default) auto-clears it.
 */
export function useTransientMessage(defaultDurationMs = 5000): {
  message: TransientMessage | null;
  showMessage: (text: string, opts?: ShowOptions) => void;
  clearMessage: () => void;
} {
  const [message, setMessage] = useState<TransientMessage | null>(null);
  const timerRef = useRef<number | null>(null);

  const clearMessage = useCallback(() => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    setMessage(null);
  }, []);

  const showMessage = useCallback(
    (text: string, opts?: ShowOptions) => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current);
      setMessage({ text, tone: opts?.tone ?? "info" });
      const duration = opts?.durationMs ?? defaultDurationMs;
      if (duration > 0) {
        timerRef.current = window.setTimeout(() => {
          timerRef.current = null;
          setMessage(null);
        }, duration);
      }
    },
    [defaultDurationMs]
  );

  useEffect(
    () => () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    },
    []
  );

  return { message, showMessage, clearMessage };
}