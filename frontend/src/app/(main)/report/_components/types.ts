export interface ReportStats {
  total_agents?: number;
  online_agents?: number;
  total_tasks?: number;
  success_tasks?: number;
  failed_tasks?: number;
  total_creds?: number;
  total_audits?: number;
  total_listeners?: number;
  total_findings?: number;
  critical_findings?: number;
  high_findings?: number;
  medium_findings?: number;
}

export interface AgentRow {
  id?: string;
  hostname?: string;
  ip?: string;
  os?: string;
  last_seen?: string;
  status?: string;
}

export interface TaskStatRow {
  type?: string;
  total?: number;
  success?: number;
  failed?: number;
  success_rate?: number;
}

export interface CredRow {
  type?: string;
  count?: number;
  source?: string;
}

export interface ListenerRow {
  id?: string;
  name?: string;
  protocol?: string;
  status?: string;
  agent_count?: number;
  traffic?: string;
}

export interface FindingRow {
  id?: string;
  title?: string;
  severity?: string;
  cve_id?: string;
  description?: string;
  recommendation?: string;
}

export interface ReportHistoryRow {
  id?: string;
  template?: string;
  format?: string;
  created_at?: string;
  sections?: string[];
  size?: string;
}

export function severityColor(s: string): string {
  if (s === "critical") return "bg-destructive/10 text-destructive border-destructive/20";
  if (s === "high") return "bg-warning/10 text-warning border-warning/20";
  if (s === "medium") return "bg-warning/10 text-warning border-warning/20";
  if (s === "low") return "bg-info/10 text-info border-info/20";
  return "bg-secondary/50 text-muted-foreground border-border";
}

export function computeDateRange(
  datePreset: string,
  customStart: string,
  customEnd: string,
): { start: string; end: string } {
  if (datePreset === "custom") {
    return { start: customStart, end: customEnd };
  }
  const days = parseInt(datePreset, 10);
  const end = new Date();
  const start = new Date();
  start.setDate(start.getDate() - days);
  return {
    start: start.toISOString().split("T")[0],
    end: end.toISOString().split("T")[0],
  };
}
