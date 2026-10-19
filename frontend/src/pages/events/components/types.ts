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

export type EventsTab = "stream" | "tasks" | "alerts" | "traffic";
export const EVENTS_TABS: readonly EventsTab[] = ["stream", "tasks", "alerts", "traffic"];

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
