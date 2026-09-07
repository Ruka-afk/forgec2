export type AITraceStage = "analyzing" | "reasoning" | "tool" | "synthesizing" | "answering";
export type AITraceStepStatus = "active" | "complete" | "error";
export type AITraceStatus = "running" | "complete" | "error";
export const AI_TRACE_TOOL_NAME = "__ai_trace__";
export const AI_REGENERATE_TOOL_NAME = "__ai_regenerate__";
export const AI_ERROR_TOOL_NAME = "__ai_error__";

export interface AITraceStep {
  id: string;
  stage: AITraceStage;
  status: AITraceStepStatus;
  tool_name?: string;
  tool_call_id?: string;
  started_at: number;
  completed_at?: number;
}

export interface AIMessage {
	id?: number;
  created_at?: string;
  role: "user" | "assistant" | "tool";
  content: string;
  tool_name?: string;
  tool_call_id?: string;
  thinking?: boolean;
  stream_id?: string;
  trace?: AITraceStep[];
  trace_status?: AITraceStatus;
  reasoning?: string;
  regenerated?: boolean;
  error?: boolean;
  run_status?: "streaming" | "complete" | "error" | "interrupted";
  duration_ms?: number;
  prompt_tokens?: number;
  completion_tokens?: number;
  truncated?: boolean;
  tool_status?: "running" | "success" | "error" | "waiting_approval";
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
	profile_id?: number;
	pinned?: boolean;
	archived?: boolean;
	parent_session_id?: number;
	write_policy?: "approval" | "low_risk_auto";
	draft?: string;
}
