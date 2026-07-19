export interface Agent {
  id?: string;
  hostname?: string;
  ip?: string;
  os?: string;
  arch?: string;
  username?: string;
  status?: string;
  last_seen?: string;
}

export interface AgentSummary {
  id?: number;
  agent_id?: string;
  hostname?: string;
  internal_ip?: string;
  external_ip?: string;
  os?: string;
  arch?: string;
  online?: boolean;
  last_seen?: string;
  listener_id?: string;
}

export interface AgentDetail {
  id?: string;
  hostname?: string;
  ip?: string;
  public_ip?: string;
  os?: string;
  arch?: string;
  username?: string;
  integrity?: string;
  elevated?: boolean;
  domain?: string;
  country?: string;
  city?: string;
  pid?: number;
  process_name?: string;
  version?: string;
  latitude?: number;
  longitude?: number;
  current_interval?: number;
  current_jitter?: number;
  last_seen?: string;
  active_window?: string;
  parent_id?: string;
  peer_count?: number;
  created_at?: string;
  listener_id?: string | number;
  tags?: string;
  status?: string;
  kill_date?: string;
}

export interface AgentDetailData {
  agent?: AgentDetail;
  tasks?: AgentTaskRecord[];
  uptime?: string;
  time_since_last_seen?: string;
  success_rate?: number;
}

export interface AgentTaskRecord {
  id?: number;
  type?: string;
  command?: string;
  status?: string;
  result?: string;
  created_at?: string;
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
  status: string;
  last_seen: string;
  listener_id: string;
  tags: string;
}

export interface DashboardStats {
  online_agents?: number;
  total_agents?: number;
  total_listeners?: number;
  total_tasks?: number;
  total_creds?: number;
  total_credentials?: number;
  total_audits?: number;
  today_tasks?: number;
  pending_tasks?: number;
  failed_tasks?: number;
  completed_tasks?: number;
  online_count?: number;
  stale_count?: number;
  offline_count?: number;
  listener_count?: number;
  pending_count?: number;
  online_users?: number;
  server_version?: string;
  recent_tasks?: { status: string; type: string; command: string; created_at: string }[];
}
