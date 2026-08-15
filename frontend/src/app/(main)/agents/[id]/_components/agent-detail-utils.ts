import type { AgentStatus } from "@/types/agent";
import { Apple, Monitor, Terminal } from "lucide-react";

export interface AgentDetailModel {
  ID?: string; id?: string;
  Hostname?: string; hostname?: string;
  IP?: string; ip?: string;
  PublicIP?: string; public_ip?: string;
  OS?: string; os?: string;
  Arch?: string; arch?: string;
  Version?: string; version?: string;
  Status?: string; status?: string;
  LastSeen?: string; last_seen?: string;
  CreatedAt?: string; created_at?: string;
  Note?: string; note?: string;
  Notes?: string; notes?: string;
  Username?: string; username?: string;
  Tags?: string; tags?: string;
  PID?: number; pid?: number;
  ProcessName?: string; process_name?: string;
  Integrity?: string; integrity?: string;
  Elevated?: boolean; elevated?: boolean;
  Domain?: string; domain?: string;
  Country?: string; country?: string;
  City?: string; city?: string;
  Latitude?: number; latitude?: number;
  Longitude?: number; longitude?: number;
  ListenerID?: number; listener_id?: number;
  CurrentInterval?: number; current_interval?: number;
  CurrentJitter?: number; current_jitter?: number;
  ActiveWindow?: string; active_window?: string;
  ParentID?: string; parent_id?: string;
  P2PMode?: string; p2p_mode?: string;
  PeerCount?: number; peer_count?: number;
  KillDate?: string; kill_date?: string;
}

export interface TaskEntry {
  ID?: number; id?: number;
  Type?: string; type?: string;
  Command?: string; command?: string;
  Status?: string; status?: string;
  Result?: string; result?: string;
  Error?: string; error?: string;
  CreatedAt?: string; created_at?: string;
  CreatedBy?: string; created_by?: string;
  UpdatedAt?: string; updated_at?: string;
}

export interface LogEntry {
  id?: string; ID?: string;
  user?: string;
  created_at?: string; CreatedAt?: string;
  message?: string;
  type?: string;
}

export interface AgentDetailResponse {
  agent?: AgentDetailModel;
  tasks?: TaskEntry[];
  screenshots?: string[];
  logs?: LogEntry[];
  total_tasks?: number;
  completed_tasks?: number;
  pending_tasks?: number;
  failed_tasks?: number;
  success_rate?: number;
  avg_response_time?: string;
  shell_tasks?: number;
  screenshot_tasks?: number;
  ps_tasks?: number;
  kill_tasks?: number;
  uptime?: string;
  time_since_last_seen?: string;
  children?: AgentDetailModel[];
}

export function getOSIcon(os: string): typeof Monitor {
  switch (os.toLowerCase()) {
    case "windows": return Monitor;
    case "linux": return Terminal;
    case "darwin":
    case "macos": return Apple;
    default: return Monitor;
  }
}

export function buildAgentMarkdown(
  data: AgentDetailResponse,
): string {
  const agent = data.agent || {};
  const hostname = agent.hostname || "—";
  const agentID = agent.id || "";
  const ip = agent.ip || "—";
  const publicIP = agent.public_ip || "";
  const os = agent.os || "—";
  const arch = agent.arch || "—";
  const username = agent.username || "—";
  const status = (agent.status || "offline") as AgentStatus;
  const uptime = data.uptime || "—";
  const totalTasks = data.total_tasks ?? 0;
  const completedTasks = data.completed_tasks ?? 0;
  const pendingTasks = data.pending_tasks ?? 0;
  const failedTasks = data.failed_tasks ?? 0;
  return [
    `# Agent: ${hostname}`,
    "",
    `| Field | Value |`,
    `|-------|-------|`,
    `| Agent ID | ${agentID} |`,
    `| Hostname | ${hostname} |`,
    `| OS | ${os} ${arch} |`,
    `| IP | ${ip} |`,
    `| Public IP | ${publicIP || "—"} |`,
    `| User | ${username} |`,
    `| Status | ${status} |`,
    `| Uptime | ${uptime} |`,
    `| Tasks | ${totalTasks} (${completedTasks} completed, ${pendingTasks} pending, ${failedTasks} failed) |`,
    "",
  ].join("\n");
}

export function buildAgentCopyText(data: AgentDetailResponse): string {
  const agent = data.agent || {};
  const hostname = agent.hostname || "—";
  const agentID = agent.id || "";
  const ip = agent.ip || "—";
  const publicIP = agent.public_ip || "";
  const os = agent.os || "—";
  const arch = agent.arch || "—";
  const username = agent.username || "—";
  const status = (agent.status || "offline") as AgentStatus;
  const uptime = data.uptime || "—";
  const totalTasks = data.total_tasks ?? 0;
  const completedTasks = data.completed_tasks ?? 0;
  const pendingTasks = data.pending_tasks ?? 0;
  const failedTasks = data.failed_tasks ?? 0;
  return [
    `Agent: ${hostname} (${agentID})`,
    `OS: ${os} ${arch}`,
    `IP: ${ip}${publicIP ? ` (public: ${publicIP})` : ""}`,
    `User: ${username}`,
    `Status: ${status}`,
    `Uptime: ${uptime}`,
    `Tasks: ${totalTasks} (${completedTasks} completed, ${pendingTasks} pending, ${failedTasks} failed)`,
  ].join("\n");
}
