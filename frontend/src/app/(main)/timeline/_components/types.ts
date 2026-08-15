export interface TimelineEvent {
  id?: string;
  timestamp?: string;
  type?: string;
  title?: string;
  description?: string;
  username?: string;
  agent_id?: string;
  url?: string;
}

export const EVENT_TYPES = ["agent_online", "task", "credential", "user", "system", "alert"] as const;

export const EVENT_COLORS: Record<string, { dot: string; bg: string; text: string }> = {
  agent_online: { dot: "bg-success", bg: "bg-success/15", text: "text-success" },
  task: { dot: "bg-info", bg: "bg-info/15", text: "text-info" },
  credential: { dot: "bg-warning", bg: "bg-warning/15", text: "text-warning" },
  user: { dot: "bg-chart-6", bg: "bg-chart-6/15", text: "text-chart-6" },
  system: { dot: "bg-chart-6", bg: "bg-chart-6/15", text: "text-chart-6" },
  alert: { dot: "bg-destructive", bg: "bg-destructive/15", text: "text-destructive" },
};

export type EventsTab = "stream" | "tasks" | "alerts";
export const EVENTS_TABS: readonly EventsTab[] = ["stream", "tasks", "alerts"];

export type UnifiedSource = "timeline" | "task" | "alert";

export interface UnifiedEvent {
  id: string;
  at: string;
  source: UnifiedSource;
  kind: string;
  title: string;
  detail: string;
  agentId?: string;
  href?: string;
  status?: string;
}
