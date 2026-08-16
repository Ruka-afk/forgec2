import type { AgentStatus } from "@/types/agent";
import { Monitor, Terminal, Apple, Laptop, type LucideIcon } from "lucide-react";

export interface TaskStats {
  pending: number;
  running: number;
  completed: number;
  failed: number;
}

export interface Beacon {
  id?: string;
  hostname?: string;
  username?: string;
  ip?: string;
  os?: string;
  arch?: string;
  status?: AgentStatus;
  last_seen?: string;
  integrity?: string;
  elevated?: boolean;
  notes?: string;
  active_window?: string;
  version?: string;
  parent_id?: string;
  public_ip?: string;
  domain?: string;
  country?: string;
  current_interval?: number;
  current_jitter?: number;
  pid?: number;
  process_name?: string;
  created_at?: string;
  taskStats?: TaskStats;
}

export interface BulkResult {
  id: string;
  type: string;
  command: string;
  agent_ids: string[];
  task_count: number;
  failed: string[];
  created_at: string;
}

export interface Tag {
  id: string;
  name: string;
  color: string;
}

export const COMMAND_TYPES = [
  { value: "shell", labelKey: "command.type.shell" },
  { value: "ps", labelKey: "command.type.process_list" },
  { value: "ls", labelKey: "command.type.list_directory" },
  { value: "screenshot", labelKey: "command.type.screenshot" },
  { value: "sleep", labelKey: "command.type.sleep" },
  { value: "netstat", labelKey: "command.type.netstat" },
  { value: "users", labelKey: "command.type.users" },
  { value: "clipboard_get", labelKey: "command.type.clipboard_get" },
  { value: "creds_dump", labelKey: "command.type.creds_dump" },
  { value: "run_evasion", labelKey: "command.type.run_evasion" },
  { value: "kill", labelKey: "command.type.kill" },
  { value: "uninstall", labelKey: "command.type.uninstall" },
];

export function avatarColor(hostname: string) {
  const colors = ["bg-primary/100", "bg-success", "bg-warning", "bg-chart-5", "bg-chart-2"];
  let h = 0;
  for (let i = 0; i < hostname.length; i++) h = hostname.charCodeAt(i) + ((h << 5) - h);
  return colors[Math.abs(h) % colors.length];
}

export function copyToClipboard(text: string, key: string, setCopied: (k: string) => void) {
  navigator.clipboard.writeText(text).then(() => { setCopied(key); setTimeout(() => setCopied(""), 1500); }).catch(() => {});
}

export function formatUptime(dateStr: string, t?: (key: string, params?: Record<string, string | number>) => string): string {
  if (!dateStr) return t?.("time.ago.unknown") ?? "";
  const diff = Date.now() - new Date(dateStr).getTime();
  if (diff < 0) return t?.("time.ago.unknown") ?? "";
  const d = Math.floor(diff / 86400000);
  const h = Math.floor((diff % 86400000) / 3600000);
  const m = Math.floor((diff % 3600000) / 60000);
  if (d > 0) return t?.("format.uptime.days", { n: d, h, m }) ?? `${d}d ${h}h`;
  if (h > 0) return t?.("format.uptime.hours", { n: h, m }) ?? `${h}h ${m}m`;
  return t?.("format.uptime.minutes", { n: m }) ?? `${m}m`;
}

/** Left status accent for agent table/grid rows. Single source of truth so
 *  AgentRow and AgentGrid render the same status border. Handles both the
 *  beacon status vocabulary (active/idle/lost) and host-group vocabulary
 *  (online/stale/lost). */
export function agentStatusBorderClass(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "active":
    case "online":
      return "border-l-2 border-l-success";
    case "idle":
    case "stale":
      return "border-l-2 border-l-warning";
    case "lost":
    case "offline":
      return "border-l-2 border-l-destructive";
    default:
      return "border-l-2 border-l-border";
  }
}

/** Maps an OS string to its Lucide icon component. */
export function osIcon(os?: string): LucideIcon {
  const o = (os || "").toLowerCase();
  if (o.includes("win")) return Monitor;
  if (o.includes("linux")) return Terminal;
  if (o.includes("mac") || o.includes("darwin")) return Apple;
  return Laptop;
}

/** Badge variant for agent integrity level. */
export function integrityTone(integrity?: string): "default" | "secondary" | "destructive" | "outline" | "warning" | "success" {
  switch ((integrity || "").toLowerCase()) {
    case "high":
      return "destructive";
    case "medium":
      return "warning";
    case "low":
      return "secondary";
    default:
      return "outline";
  }
}
