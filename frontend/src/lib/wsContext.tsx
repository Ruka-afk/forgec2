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

// Module-level listeners so non-React code (e.g. the task poller in api.ts)
// can react to WS frames without being inside a component/hook.
const globalListeners = new Set<WSListener>();

// activeWS is the current operator WebSocket, used to send control frames
// (subscribe/unsubscribe) from anywhere without a hook.
let activeWS: WebSocket | null = null;

// Track subscribed agents so we can re-subscribe after reconnection
const subscribedAgents = new Set<string>();

// sendWSMessage sends a control frame to the server if the socket is open.
export function sendWSMessage(obj: unknown): void {
  if (activeWS && activeWS.readyState === WebSocket.OPEN) {
    activeWS.send(JSON.stringify(obj));
  }
}

// subscribeAgent tells the server to scope agent-scoped broadcasts to this connection.
export function subscribeAgent(agentID: string): void {
  subscribedAgents.add(agentID);
  sendWSMessage({ type: "subscribe", agent_id: agentID });
}

// unsubscribeAgent removes a previous subscription for agentID.
export function unsubscribeAgent(agentID: string): void {
  subscribedAgents.delete(agentID);
  sendWSMessage({ type: "unsubscribe", agent_id: agentID });
}

// onWSMessage registers a global WS message listener and returns an unsubscribe fn.
export function onWSMessage(listener: WSListener): () => void {
  globalListeners.add(listener);
  return () => {
    globalListeners.delete(listener);
  };
}

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
  const envURL = process.env.NEXT_PUBLIC_WS_URL;
  if (envURL) return envURL + path;
  const proto = typeof window !== "undefined" && window.location.protocol === "https:" ? "wss:" : "ws:";
  const host = typeof window !== "undefined" ? window.location.hostname : "localhost";
  const port = process.env.NEXT_PUBLIC_GO_BACKEND_PORT || "8443";
  return `${proto}//${host}:${port}${path}`;
}

const MAX_RECONNECT_DELAY = 30000;
const MAX_RECONNECT_ATTEMPTS = 20;

export function WebSocketProvider({ children }: { children: ReactNode }) {
  const [connected, setConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<WSMessage | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const listenersRef = useRef<Set<WSListener>>(new Set());
  const reconnectAttemptRef = useRef(0);

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
      activeWS = ws;

      ws.onopen = () => {
        reconnectAttemptRef.current = 0;
        const token = getCookie("forgec2_session");
        if (token) {
          ws.send(JSON.stringify({ type: "auth", token }));
        }
        // Re-subscribe to previously subscribed agents
        for (const agentID of subscribedAgents) {
          ws.send(JSON.stringify({ type: "subscribe", agent_id: agentID }));
        }
        setConnected(true);
      };
      ws.onclose = () => {
        setConnected(false);
        if (reconnectAttemptRef.current < MAX_RECONNECT_ATTEMPTS) {
          const delay = Math.min(1000 * Math.pow(2, reconnectAttemptRef.current), MAX_RECONNECT_DELAY);
          reconnectAttemptRef.current++;
          reconnectRef.current = setTimeout(connect, delay);
        }
      };
      ws.onerror = () => ws.close();
      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WSMessage;
          setLastMessage(msg);
          listenersRef.current.forEach((fn) => fn(msg));
          globalListeners.forEach((fn) => fn(msg));
        } catch {
          // Silently ignore malformed WS frames
        }
      };
    };

    connect();

    return () => {
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
      activeWS = null;
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
