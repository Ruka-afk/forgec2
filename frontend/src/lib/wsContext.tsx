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
import { DEFAULT_WS_HOST, DEFAULT_WS_PORT } from "./constants";

export interface WSMessage {
  type: string;
  [key: string]: unknown;
}

type WSListener = (msg: WSMessage) => void;

// Module-level listeners so non-React code (e.g. the task poller in api.ts)
// can observe WS messages without a React context. This is a second, distinct
// channel from the provider-level `listenersRef` set below — subscribers are
// never duplicated across channels, and each channel stays idempotent because
// both are backed by Sets (re-registering the same listener is a no-op).
const globalListeners = new Set<WSListener>();

// onWSMessage registers a global WS message listener and returns an unsubscribe fn.
export function onWSMessage(listener: WSListener): () => void {
  globalListeners.add(listener);
  return () => {
    globalListeners.delete(listener);
  };
}

interface WSContextValue {
  connected: boolean;
  reconnectFailed: boolean;
  subscribe: (listener: WSListener) => () => void;
  send: (data: unknown) => void;
  reconnect: () => void;
}

const WSContext = createContext<WSContextValue | null>(null);

function getCookie(name: string): string | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(new RegExp("(?:^|; )" + name.replace(/([$?*|{}[\]()\\^])/g, "\\$1") + "=([^;]*)"));
  return match ? decodeURIComponent(match[1]) : null;
}

export function getWSURL(path = "/ws"): string {
  const envURL = process.env.NEXT_PUBLIC_WS_URL;
  if (envURL) return envURL + path;
  const proto = typeof window !== "undefined" && window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = typeof window !== "undefined" ? window.location.hostname : DEFAULT_WS_HOST;
  let port = process.env.NEXT_PUBLIC_GO_BACKEND_PORT || (typeof window !== "undefined" ? window.location.port : DEFAULT_WS_PORT);
  if (!port) port = proto === "wss:" ? "443" : "80";
  return `${proto}//${host}:${port}${path}`;
}

const MAX_RECONNECT_DELAY = 30000;
const MAX_RECONNECT_ATTEMPTS = 20;
const BACKGROUND_RETRY_DELAY_MS = 60000;
const HEARTBEAT_INTERVAL_MS = 30000;
const HEARTBEAT_TIMEOUT_MS = 60000;
const MAX_BUFFER_SIZE = 50;

export function WebSocketProvider({ children }: { children: ReactNode }) {
  const [connected, setConnected] = useState(false);
  const [reconnectFailed, setReconnectFailed] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const heartbeatRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const lastPongRef = useRef(0);
  const listenersRef = useRef<Set<WSListener>>(new Set());
  const reconnectAttemptRef = useRef(0);
  const sendBufferRef = useRef<string[]>([]);

  const subscribe = useCallback((listener: WSListener) => {
    listenersRef.current.add(listener);
    return () => {
      listenersRef.current.delete(listener);
    };
  }, []);

  const send = useCallback((data: unknown) => {
    const raw = JSON.stringify(data);
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(raw);
    } else {
      if (sendBufferRef.current.length >= MAX_BUFFER_SIZE) {
        sendBufferRef.current.shift();
      }
      sendBufferRef.current.push(raw);
    }
  }, []);

  useEffect(() => {
    let disposed = false;
    const startHeartbeat = () => {
      lastPongRef.current = Date.now();
      heartbeatRef.current = setInterval(() => {
        const ws = wsRef.current;
        if (!ws || ws.readyState !== WebSocket.OPEN) return;
        if (Date.now() - lastPongRef.current > HEARTBEAT_TIMEOUT_MS) {
          ws.close();
          return;
        }
        ws.send(JSON.stringify({ type: "ping" }));
      }, HEARTBEAT_INTERVAL_MS);
    };

    const stopHeartbeat = () => {
      if (heartbeatRef.current) { clearInterval(heartbeatRef.current); heartbeatRef.current = null; }
    };

    const connect = () => {
      if (disposed) return;
      if (reconnectRef.current) { clearTimeout(reconnectRef.current); reconnectRef.current = null; }
      if (wsRef.current && (wsRef.current.readyState === WebSocket.OPEN || wsRef.current.readyState === WebSocket.CONNECTING)) return;

      // Fan a message out to both subscription channels. Listeners are
      // fire-and-forget: a throwing listener is skipped so one bad subscriber
      // cannot break the bus.
      const dispatch = (msg: WSMessage) => {
        for (const fn of listenersRef.current) {
          try { fn(msg); } catch { /* listener error: skip */ }
        }
        for (const fn of globalListeners) {
          try { fn(msg); } catch { /* listener error: skip */ }
        }
      };

      const ws = new WebSocket(getWSURL());
      wsRef.current = ws;

      ws.onopen = () => {
        reconnectAttemptRef.current = 0;
        setReconnectFailed(false);
        while (sendBufferRef.current.length > 0) {
          const buffered = sendBufferRef.current.shift()!;
          ws.send(buffered);
        }
        setConnected(true);
        startHeartbeat();
      };
      ws.onclose = () => {
        stopHeartbeat();
        setConnected(false);
        if (disposed) return;
        if (!getCookie("forgec2_session")) {
          setReconnectFailed(true);
          if (typeof window !== "undefined" && window.location.pathname !== "/login") {
            window.location.href = "/login";
          }
          return;
        }
        if (reconnectAttemptRef.current < MAX_RECONNECT_ATTEMPTS) {
          const delay = Math.min(1000 * Math.pow(2, reconnectAttemptRef.current), MAX_RECONNECT_DELAY);
          reconnectAttemptRef.current++;
          reconnectRef.current = setTimeout(connect, delay);
        } else {
          setReconnectFailed(true);
          // Keep trying at a slow cadence after the fast backoff is exhausted
          // so the socket recovers without user intervention once the backend
          // is reachable again. The banner stays up until a connection succeeds.
          reconnectAttemptRef.current = 0;
          reconnectRef.current = setTimeout(connect, BACKGROUND_RETRY_DELAY_MS);
        }
      };
      ws.onerror = () => { ws.close(); };
      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WSMessage;
          if (msg.type === "pong") { lastPongRef.current = Date.now(); return; }
          dispatch(msg);
        } catch {
          // Silently ignore malformed WS frames
        }
      };
    };

    // Hidden tabs throttle intervals (down to ~1/min), so heartbeat checks
    // can go stale. On return to the foreground, probe immediately: a dead
    // socket is closed here and the reconnect path takes over.
    const onVisibility = () => {
      if (document.visibilityState !== "visible") return;
      const ws = wsRef.current;
      if (!ws || ws.readyState !== WebSocket.OPEN) return;
      if (lastPongRef.current === 0 || Date.now() - lastPongRef.current > HEARTBEAT_TIMEOUT_MS) {
        ws.close();
        return;
      }
      ws.send(JSON.stringify({ type: "ping" }));
    };
    document.addEventListener("visibilitychange", onVisibility);

    connect();

    return () => {
      disposed = true;
      stopHeartbeat();
      document.removeEventListener("visibilitychange", onVisibility);
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
      wsRef.current?.close();
    };
  }, []);

  const reconnect = useCallback(() => {
    setReconnectFailed(false);
    reconnectAttemptRef.current = 0;
    if (reconnectRef.current) { clearTimeout(reconnectRef.current); reconnectRef.current = null; }
    if (wsRef.current) wsRef.current.close();
  }, []);

  return (
    <WSContext.Provider value={{ connected, reconnectFailed, subscribe, send, reconnect }}>
      {children}
    </WSContext.Provider>
  );
}

export function useWS() {
  const ctx = useContext(WSContext);
  if (!ctx) throw new Error("useWS must be used within WebSocketProvider");
  return ctx;
}
