"use client";

import { useEffect, useRef } from "react";
import { useWS, type WSMessage } from "@/lib/wsContext";

export type { WSMessage };

export function useWebSocket(onMessage?: (msg: WSMessage) => void) {
  const { connected, subscribe, lastMessage } = useWS();
  const onMessageRef = useRef(onMessage);

  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  useEffect(() => {
    const handler = (msg: WSMessage) => onMessageRef.current?.(msg);
    return subscribe(handler);
  }, [subscribe]);

  return { connected, lastMessage };
}
