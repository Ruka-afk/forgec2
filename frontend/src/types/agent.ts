export interface AgentSummary {
  ID?: number;
  id?: number;
  AgentId?: string;
  agent_id?: string;
  Hostname?: string;
  hostname?: string;
  InternalIP?: string;
  internal_ip?: string;
  ExternalIP?: string;
  external_ip?: string;
  OS?: string;
  os?: string;
  Arch?: string;
  arch?: string;
  Online?: boolean;
  online?: boolean;
  LastSeen?: string;
  last_seen?: string;
  ListenerId?: string;
  listener_id?: string;
}

export interface DashboardStats {
  OnlineAgents?: number;
  online_agents?: number;
  TotalAgents?: number;
  total_agents?: number;
  TotalListeners?: number;
  total_listeners?: number;
  TotalTasks?: number;
  total_tasks?: number;
  TotalCredentials?: number;
  total_credentials?: number;
  TotalAudits?: number;
  total_audits?: number;
  OnlineCount?: number;
  ListenerCount?: number;
}
