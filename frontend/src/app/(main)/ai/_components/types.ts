export interface AIMessage {
  role: "user" | "assistant" | "tool";
  content: string;
  tool_name?: string;
  thinking?: boolean;
}

export interface AIConfig {
  enabled: boolean;
  has_api_key?: boolean;
  provider: string;
  api_key: string;
  model: string;
  endpoint: string;
  system_prompt: string;
  engagement_notes?: string;
  allow_execute?: boolean;
}

export interface AISession {
  id: number;
  title: string;
  updated_at: string;
}
