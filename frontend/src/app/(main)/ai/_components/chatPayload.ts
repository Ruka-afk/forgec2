import { AI_ERROR_TOOL_NAME, AI_REGENERATE_TOOL_NAME, AI_TRACE_TOOL_NAME, type AIMessage } from "./types";

const TOOL_DIGEST_EACH = 1500;
const TOOL_DIGEST_MAX = 6;
const TOOL_DIGEST_TOTAL = 8000;
export const AI_INPUT_MAX_CHARS = 16_000;
export const AI_CONTEXT_MAX_CHARS = 48_000;
const AI_CONTEXT_MAX_TURNS = 120;

export interface ChatTurn {
  role: "user" | "assistant";
  content: string;
}

export interface PersistedChatMessage {
  role: "user" | "assistant" | "tool";
  content: string;
  tool_name: string;
}

/** Serialize one visible turn for the atomic session-message endpoint. */
export function buildPersistedTurn(messages: AIMessage[]): PersistedChatMessage[] {
  return messages.flatMap((message) => {
    if (message.thinking) return [];
    const isTrace = Array.isArray(message.trace);
    return [{
      role: isTrace ? "tool" : message.role,
      content: isTrace
        ? JSON.stringify({
            trace: message.trace,
            trace_status: message.trace_status,
            reasoning: message.reasoning || "",
          })
        : message.content,
      tool_name: isTrace
        ? AI_TRACE_TOOL_NAME
        : message.regenerated
          ? AI_REGENERATE_TOOL_NAME
          : message.error
            ? AI_ERROR_TOOL_NAME
            : (message.tool_name || ""),
    }];
  });
}

function toolBody(content: string): string {
  const trimmed = content.trim();
  const nl = trimmed.indexOf("\n");
  const body = (nl >= 0 ? trimmed.slice(nl + 1) : trimmed).trim();
  if (body.length <= TOOL_DIGEST_EACH) return body;
  return `${body.slice(0, TOOL_DIGEST_EACH)}…`;
}

function flushTools(tools: { name: string; body: string }[]): ChatTurn | null {
  if (tools.length === 0) return null;
  const kept = tools.slice(-TOOL_DIGEST_MAX);
  let digest = kept.map((t) => `${t.name}: ${t.body}`).join("\n");
  if (digest.length > TOOL_DIGEST_TOTAL) {
    digest = `${digest.slice(0, TOOL_DIGEST_TOTAL)}…`;
  }
  return {
    role: "user",
    content: `[Tool results — reuse unless the operator asks to refresh]\n${digest}`,
  };
}

function clipContextTurn(turn: ChatTurn): ChatTurn {
  if (turn.content.length <= AI_INPUT_MAX_CHARS) return turn;
  return { ...turn, content: `${turn.content.slice(0, AI_INPUT_MAX_CHARS)}…` };
}

function capConversationPayload(turns: ChatTurn[]): ChatTurn[] {
  const clipped = turns.map(clipContextTurn);
  const kept: ChatTurn[] = [];
  const noticeReserve = 180;
  let remaining = AI_CONTEXT_MAX_CHARS - noticeReserve;
  let i = clipped.length - 1;
  for (; i >= 0 && kept.length < AI_CONTEXT_MAX_TURNS; i--) {
    const turn = clipped[i];
    if (turn.content.length > remaining) break;
    kept.unshift(turn);
    remaining -= turn.content.length;
  }
  const removed = i + 1;
  if (removed > 0) {
    kept.unshift({
      role: "user",
      content: `[System] ${removed} earlier conversation turns were omitted to keep this request responsive.`,
    });
  }
  return kept;
}

/** Build the OpenAI-style message list for /ai/chat. Tool outputs are folded
 *  into a compact user digest so follow-up turns keep live C2 facts without
 *  sending invalid `role:tool` frames (those need matching tool_calls). */
export function buildConversationPayload(messages: AIMessage[]): ChatTurn[] {
  const out: ChatTurn[] = [];
  let pending: { name: string; body: string }[] = [];

  const emitTools = () => {
    const block = flushTools(pending);
    pending = [];
    if (block) out.push(block);
  };

  for (const message of messages) {
    if (message.thinking || message.trace) continue;
    if (message.role === "tool") {
      pending.push({
        name: message.tool_name || "tool",
        body: toolBody(message.content || ""),
      });
      continue;
    }
    const text = (message.content || "").trim();
    if (!text) continue;
    if (message.role === "assistant") emitTools();
    const last = out[out.length - 1];
    if (last && last.role === message.role && last.content === text) continue;
    out.push({ role: message.role, content: text });
  }
  emitTools();
  return capConversationPayload(out);
}

/** Short sidebar title: map canned quick-action prompts to their labels,
 *  otherwise the first line, capped. */
export function sessionTitleFromQuery(
  text: string,
  canned: { query: string; label: string }[],
): string {
  const trimmed = text.trim();
  if (!trimmed) return "New Chat";
  const hit = canned.find((item) => item.query === trimmed);
  if (hit) return hit.label;
  const line = trimmed.split("\n")[0].trim();
  return line.length <= 36 ? line : `${line.slice(0, 36)}…`;
}
