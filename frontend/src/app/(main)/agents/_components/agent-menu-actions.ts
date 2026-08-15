import type { Beacon } from "./types";

export type AgentMenuAction =
  | "interact"
  | "details"
  | "screenshot"
  | "sleep"
  | "notes"
  | "files"
  | "socks"
  | "copy_id"
  | "tokens"
  | "screen"
  | "kill"
  | "uninstall"
  | "delete";

export interface AgentMenuPoint {
  x: number;
  y: number;
  beacon: Beacon;
}

export function clampMenuPoint(x: number, y: number, width = 220, height = 360): { x: number; y: number } {
  if (typeof window === "undefined") return { x, y };
  const pad = 8;
  return {
    x: Math.min(Math.max(pad, x), window.innerWidth - width - pad),
    y: Math.min(Math.max(pad, y), window.innerHeight - height - pad),
  };
}
