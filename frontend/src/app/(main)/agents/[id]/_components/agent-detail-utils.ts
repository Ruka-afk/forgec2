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

export function computeHealthScore(
  status: string,
  successRate: number,
  lastSeen: string,
  tasks: TaskEntry[],
): number {
  let score = 0;
  if (status === "online") score += 40;
  else return 0;
  score += Math.round((successRate / 100) * 30);
  if (lastSeen) {
    const diffMs = Date.now() - new Date(lastSeen).getTime();
    const diffMin = diffMs / 60000;
    if (diffMin < 5) score += 15;
    else if (diffMin < 60) score += 10;
    else if (diffMin < 1440) score += 5;
  }
  const recentFailed = tasks.slice(0, 10).some((t) => (t.status) === "failed");
  if (!recentFailed) score += 15;
  return Math.min(100, Math.max(0, score));
}

export function computeActivityBuckets(tasks: TaskEntry[], now: number): { activityBuckets: number[]; maxActivity: number } {
  const buckets: number[] = Array.from({ length: 24 }, () => 0);
  const oneDayAgo = now - 24 * 60 * 60 * 1000;
  for (const t of tasks) {
    const created = t.created_at;
    if (!created) continue;
    const ts = new Date(created).getTime();
    if (ts < oneDayAgo) continue;
    const bucketIndex = Math.floor(((ts - oneDayAgo) / (24 * 60 * 60 * 1000)) * 24);
    if (bucketIndex >= 0 && bucketIndex < 24) buckets[bucketIndex]++;
  }
  return { activityBuckets: buckets, maxActivity: Math.max(...buckets, 1) };
}

export function computeSparklinePoints(tasks: TaskEntry[]): { x: number; y: number; dur: number }[] {
  const completedTasksList = tasks.filter((t) => (t.status) === "completed");
  const last10 = completedTasksList.slice(0, 10).reverse();
  if (last10.length === 0) return [];
  const durations = last10.map((t) => {
    const created = new Date(t.created_at || "").getTime();
    const updated = new Date(t.updated_at || "").getTime();
    if (!created || !updated || updated <= created) return 1000;
    return updated - created;
  });
  const maxDur = Math.max(...durations, 1);
  const minDur = Math.min(...durations, 0);
  const range = maxDur - minDur || 1;
  return durations.map((dur, i) => ({
    x: (i / Math.max(durations.length - 1, 1)) * 100,
    y: 20 - ((dur - minDur) / range) * 18,
    dur,
  }));
}

export function buildAgentMarkdown(
  data: AgentDetailResponse,
  healthScore: number,
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
    `| Health Score | ${healthScore}/100 |`,
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
