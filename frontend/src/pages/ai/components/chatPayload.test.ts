import { describe, expect, it } from "vitest";
import { AI_CONTEXT_MAX_CHARS, buildConversationPayload, buildPersistedTurn, sessionTitleFromQuery } from "./chatPayload";
import { AI_ERROR_TOOL_NAME, AI_REGENERATE_TOOL_NAME, AI_TRACE_TOOL_NAME } from "./types";
import type { AIMessage } from "./types";

describe("buildConversationPayload", () => {
  it("folds tool results into a digest before the assistant answer", () => {
    const messages: AIMessage[] = [
      { role: "user", content: "态势" },
      { role: "assistant", content: "", trace: [], trace_status: "complete" },
      { role: "tool", content: "结果:\n{\"agents_online\":2}", tool_name: "get_situation" },
      { role: "assistant", content: "当前 2 台上线。" },
    ];
    const payload = buildConversationPayload(messages);
    expect(payload.map((m) => m.role)).toEqual(["user", "user", "assistant"]);
    expect(payload[1].content).toContain("[Tool results");
    expect(payload[1].content).toContain("get_situation");
    expect(payload[1].content).toContain("agents_online");
    expect(payload[2].content).toContain("2 台上线");
  });

  it("keeps tool facts for a follow-up turn", () => {
    const messages: AIMessage[] = [
      { role: "user", content: "态势" },
      { role: "tool", content: "结果:\n{\"agents_online\":1}", tool_name: "get_situation" },
      { role: "assistant", content: "1 online" },
      { role: "user", content: "下一步" },
    ];
    const payload = buildConversationPayload(messages);
    expect(payload[payload.length - 1]).toEqual({ role: "user", content: "下一步" });
    expect(payload.some((m) => m.content.includes("agents_online"))).toBe(true);
  });

  it("skips traces, thinking, and duplicate consecutive lines", () => {
    const messages: AIMessage[] = [
      { role: "user", content: "hi" },
      { role: "user", content: "hi" },
      { role: "assistant", content: "", thinking: true },
      { role: "assistant", content: "hello" },
    ];
    expect(buildConversationPayload(messages)).toEqual([
      { role: "user", content: "hi" },
      { role: "assistant", content: "hello" },
    ]);
  });

  it("keeps long sessions within the request budget and preserves the latest turn", () => {
    const messages: AIMessage[] = Array.from({ length: 180 }, (_, index) => ({
      role: index % 2 === 0 ? "user" : "assistant",
      content: `${index}:` + "x".repeat(1000),
    }));
    messages.push({ role: "user", content: "latest operator question" });

    const payload = buildConversationPayload(messages);
    expect(payload.reduce((total, turn) => total + turn.content.length, 0)).toBeLessThanOrEqual(AI_CONTEXT_MAX_CHARS);
    expect(payload[0].content).toContain("earlier conversation turns");
    expect(payload[payload.length - 1].content).toBe("latest operator question");
    expect(payload.some((turn) => turn.content.startsWith("0:"))).toBe(false);
  });
});

describe("sessionTitleFromQuery", () => {
  it("uses the canned label for a quick-action prompt", () => {
    expect(sessionTitleFromQuery("long prompt text", [
      { query: "long prompt text", label: "态势简报" },
    ])).toBe("态势简报");
  });

  it("falls back to a clipped first line", () => {
    expect(sessionTitleFromQuery("hello world", [])).toBe("hello world");
    expect(sessionTitleFromQuery("x".repeat(50), []).length).toBe(37);
  });
});

describe("buildPersistedTurn", () => {
  it("serializes a complete turn for one atomic save request", () => {
    const turn = buildPersistedTurn([
      { role: "user", content: "retry", regenerated: true },
      {
        role: "assistant",
        content: "",
        trace: [{ id: "1", stage: "reasoning", status: "complete", started_at: 1, completed_at: 2 }],
        trace_status: "complete",
        reasoning: "thought",
      },
      { role: "tool", content: "result", tool_name: "get_situation" },
      { role: "assistant", content: "answer" },
      { role: "assistant", content: "interrupted", error: true },
    ]);

    expect(turn).toHaveLength(5);
    expect(turn[0].tool_name).toBe(AI_REGENERATE_TOOL_NAME);
    expect(turn[1].role).toBe("tool");
    expect(turn[1].tool_name).toBe(AI_TRACE_TOOL_NAME);
    expect(JSON.parse(turn[1].content).reasoning).toBe("thought");
    expect(turn[2].tool_name).toBe("get_situation");
    expect(turn[3]).toMatchObject({ role: "assistant", content: "answer" });
    expect(turn[4].tool_name).toBe(AI_ERROR_TOOL_NAME);
  });
});
