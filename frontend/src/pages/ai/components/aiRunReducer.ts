import type { ParsedSSEEvent } from "@/lib/sse";

export interface AIRunViewState {
  runId: string | null;
  status: "idle" | "queued" | "running" | "waiting_approval" | "completed" | "failed" | "cancelled" | "interrupted";
  lastEventId: number;
  text: string;
  reasoning: string;
  activeTools: Record<string, string>;
  promptTokens: number;
  completionTokens: number;
  error: string;
}

export const initialAIRunViewState: AIRunViewState = {
  runId: null,
  status: "idle",
  lastEventId: 0,
  text: "",
  reasoning: "",
  activeTools: {},
  promptTokens: 0,
  completionTokens: 0,
  error: "",
};

export type AIRunViewAction =
  | { type: "started"; runId: string; status?: AIRunViewState["status"] }
  | { type: "event"; event: ParsedSSEEvent }
  | { type: "detached" };

export function aiRunViewReducer(state: AIRunViewState, action: AIRunViewAction): AIRunViewState {
  if (action.type === "detached") return initialAIRunViewState;
  if (action.type === "started") {
    return { ...initialAIRunViewState, runId: action.runId, status: action.status ?? "queued" };
  }
  const sequence = Number(action.event.id || 0);
  if (sequence > 0 && sequence <= state.lastEventId) return state;
  const next = { ...state, lastEventId: sequence > 0 ? sequence : state.lastEventId };
  const { event, data } = action.event;
  if (event === "run") {
    try {
      const parsed = JSON.parse(data) as { status?: AIRunViewState["status"] };
      next.status = parsed.status ?? "running";
    } catch { next.status = "running"; }
  } else if (event === "text") {
    next.text = data;
  } else if (event === "reasoning") {
    next.reasoning = data;
  } else if (event === "tool_start") {
    try {
      const parsed = JSON.parse(data) as { id?: string; name?: string };
      next.activeTools = { ...state.activeTools, [parsed.id || parsed.name || `tool-${sequence}`]: parsed.name || "tool" };
    } catch { /* malformed tool metadata stays visible in the legacy trace */ }
  } else if (event === "tool" || event === "tool_result") {
    next.activeTools = {};
  } else if (event === "usage") {
    try {
      const usage = JSON.parse(data) as { prompt_tokens?: number; completion_tokens?: number };
      next.promptTokens += usage.prompt_tokens || 0;
      next.completionTokens += usage.completion_tokens || 0;
    } catch { /* ignore malformed usage */ }
  } else if (event === "done") {
    next.status = "completed";
    next.reasoning = "";
  } else if (event === "error") {
    next.status = "failed";
    next.error = data;
    next.reasoning = "";
  }
  return next;
}
