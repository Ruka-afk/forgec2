import { formatTime } from "@/lib/utils";

export interface RelaySession {
  id?: string;
  agent_id: string;
  hostname?: string;
  listen_port: number;
  active: boolean;
  bytes_in: number;
  bytes_out: number;
  active_conn: number;
  conn_count: number;
  created_at: string;
}

export interface PivotAgent {
  id: string;
  hostname: string;
  ip: string;
  status: string;
}

export interface RPortForwardStatus {
  id: string;
  agent_id: string;
  remote_host: string;
  remote_port: number;
  local_port: number;
  protocol: string;
  active: boolean;
  bytes_in: number;
  bytes_out: number;
  uptime: number;
  error?: string;
}

export function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(1) + " " + sizes[i];
}

export function formatCreated(d: string): string {
  if (!d) return "-";
  return formatTime(d);
}

export function formatUptime(seconds: number): string {
  if (!seconds) return "-";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}h ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}
