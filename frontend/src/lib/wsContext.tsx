"use client";

import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  useCallback,
  type ReactNode,
} from "react";

export interface WSMessage {
  type: string;
  [key: string]: unknown;
}

type WSListener = (msg: WSMessage) => void;

interface WSContextValue {
  connected: boolean;
  subscribe: (listener: WSListener) => () => void;
  lastMessage: WSMessage | null;
}

const WSContext = createContext<WSContextValue | null>(null);

function getCookie(name: string): string | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(new RegExp("(?:^|; )" + name.replace(/([$?*|{}[\]()\\^])/g, "\\$1") + "=([^;]*)"));
  return match ? decodeURIComponent(match[1]) : null;
}

export function getWSURL(path = "/ws"): string {
  const proto = typeof window !== "undefined" && window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = typeof window !== "undefined" ? window.location.hostname : "localhost";
  const port = process.env.NEXT_PUBLIC_GO_BACKEND_PORT || "8080";
  const token = getCookie("forgec2_session");
  const qs = token ? `?token=${encodeURIComponent(token)}` : "";
  return `${proto}//${host}:${port}${path}${qs}`;
}

export function WebSocketProvider({ children }: { children: ReactNode }) {
  const [connected, setConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<WSMessage | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const listenersRef = useRef<Set<WSListener>>(new Set());

  const subscribe = useCallback((listener: WSListener) => {
    listenersRef.current.add(listener);
    return () => {
      listenersRef.current.delete(listener);
    };
  }, []);

  useEffect(() => {
    const connect = () => {
      if (wsRef.current?.readyState === WebSocket.OPEN) return;

      const ws = new WebSocket(getWSURL());
      wsRef.current = ws;

      ws.onopen = () => setConnected(true);
      ws.onclose = () => {
        setConnected(false);
        reconnectRef.current = setTimeout(connect, 3000);
      };
      ws.onerror = () => ws.close();
      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WSMessage;
          setLastMessage(msg);
          listenersRef.current.forEach((fn) => fn(msg));
        } catch {
          /* ignore malformed frames */
        }
      };
    };

    connect();

    return () => {
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
      wsRef.current?.close();
    };
  }, []);

  return (
    <WSContext.Provider value={{ connected, subscribe, lastMessage }}>
      {children}
    </WSContext.Provider>
  );
}

export function useWS() {
  const ctx = useContext(WSContext);
  if (!ctx) throw new Error("useWS must be used within WebSocketProvider");
  return ctx;
}