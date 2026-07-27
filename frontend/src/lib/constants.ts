export const API_BASE = process.env.NEXT_PUBLIC_API_BASE || "";

export const DEFAULT_WS_HOST = "localhost";
export const DEFAULT_WS_PORT = "8000";

export const DEFAULT_LISTENER_ADDR = "127.0.0.1:443";

export function agentStaticParams() {
  return [{ id: "_" }];
}

export function listenerStaticParams() {
  return [{ id: "_" }];
}
