export interface Rule {
  id?: string;
  ID?: string;
  name?: string;
  Name?: string;
  event_type?: string;
  EventType?: string;
  enabled?: boolean;
  Enabled?: boolean;
  conditions?: unknown[];
  Conditions?: unknown[];
  actions?: unknown[];
  Actions?: unknown[];
}

export interface Webhook {
  id?: number;
  ID?: number;
  name?: string;
  Name?: string;
  url?: string;
  URL?: string;
  enabled?: boolean;
  Enabled?: boolean;
  event_type?: string;
  EventType?: string;
  method?: string;
  Method?: string;
}

export interface AlertRule {
  id?: number;
  name?: string;
  type?: string;
  threshold?: number;
  enabled?: boolean;
  description?: string;
}

export interface MonitorAlert {
  id?: number;
  title?: string;
  message?: string;
  severity?: string;
  status?: string;
  source_name?: string;
  created_at?: string;
}

export type WebhookType = "generic" | "slack" | "discord" | "email";

export interface WebhookActionParams {
  type: WebhookType;
  url: string;
  secret: string;
  to: string;
  smtp_host: string;
  smtp_port: number;
  smtp_user: string;
  smtp_pass: string;
  from: string;
}

export const defaultWebhookParams: WebhookActionParams = {
  type: "generic",
  url: "",
  secret: "",
  to: "",
  smtp_host: "",
  smtp_port: 587,
  smtp_user: "",
  smtp_pass: "",
  from: "",
};
