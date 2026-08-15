export type AgentStatus = "online" | "stale" | "offline";

export type TaskStatus = "pending" | "running" | "completed" | "failed" | "cancelled" | "pending_approval";

export interface AgentBase {
  id: string;
  hostname: string;
  username: string;
  os: string;
  arch: string;
  ip: string;
  status: AgentStatus;
  last_seen: string;
  tags: string;
  listener_id: string;
  version: string;
  pid: number;
  process_name: string;
  integrity: string;
  elevated: boolean;
  domain: string;
  current_interval: number;
  current_jitter: number;
  active_window: string;
  public_ip: string;
  country: string;
  city: string;
  latitude: number;
  longitude: number;
  trusted: boolean;
  notes: string;
  parent_id: string;
  p2p_mode: string;
  p2p_listen_addr: string;
  peer_count: number;
  best_route: string;
  working_hours_start: string;
  working_hours_end: string;
  working_hours_tz: string;
  created_at: string;
  updated_at: string;
}

export type Agent = Partial<AgentBase> & { kill_date?: string };
export type RequiredAgent = Pick<AgentBase, "id" | "hostname" | "username" | "os" | "arch" | "ip" | "status" | "last_seen"> & Partial<AgentBase> & { kill_date?: string };
export type AgentSummary = Partial<AgentBase> & { taskStats?: TaskStats };
export type AgentDetail = Partial<AgentBase> & { kill_date?: string; parent_agent_id?: string };

export interface AgentDetailData {
  agent?: AgentDetail;
  tasks?: AgentTaskRecord[];
  uptime?: string;
  time_since_last_seen?: string;
  success_rate?: number;
}

export interface TaskStats {
  pending: number;
  running: number;
  completed: number;
  failed: number;
}

export interface AgentTaskRecord {
  id: number;
  type: string;
  command: string;
  status: TaskStatus;
  created_at: string;
  result?: string;
  error?: string;
  created_by?: string;
  agent_id?: string;
}

export interface NormalizedAgent {
  id: string;
  hostname: string;
  username: string;
  ip: string;
  os: string;
  status: AgentStatus;
  last_seen: string;
  listener_id: string;
  tags: string;
}

export interface DashboardStats {
  online_agents: number;
  total_agents: number;
  total_listeners: number;
  total_tasks: number;
  total_creds: number;
  total_tokens?: number;
  total_audits: number;
  today_tasks: number;
  pending_tasks: number;
  failed_tasks: number;
  completed_tasks: number;
  online_count: number;
  stale_count: number;
  offline_count: number;
  listener_count: number;
  pending_count: number;
  online_users: number;
  server_version: string;
  recent_tasks: { status: TaskStatus; type: string; command: string; created_at: string }[];
}
