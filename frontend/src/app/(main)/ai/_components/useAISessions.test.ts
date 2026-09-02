import { describe, expect, it } from "vitest";
import { AI_ERROR_TOOL_NAME, AI_REGENERATE_TOOL_NAME } from "./types";
import { restoreSessionMessages } from "./useAISessions";

describe("restoreSessionMessages", () => {
  it("restores only the latest regenerated branch", () => {
    const messages = restoreSessionMessages([
      { role: "user", content: "first" },
      { role: "assistant", content: "first answer" },
      { role: "user", content: "retry me" },
      { role: "assistant", content: "old answer" },
      { role: "user", content: "retry me", tool_name: AI_REGENERATE_TOOL_NAME },
      { role: "assistant", content: "new answer" },
    ]);

    expect(messages.map((message) => message.content)).toEqual([
      "first",
      "first answer",
      "retry me",
      "new answer",
    ]);
    expect(messages[2].regenerated).toBe(true);
  });

  it("drops invalid roles and malformed trace records", () => {
    const messages = restoreSessionMessages([
      { role: "system", content: "not allowed" },
      { role: "assistant", content: "{bad", tool_name: "__ai_trace__" },
      { role: "user", content: "valid" },
    ]);
    expect(messages).toHaveLength(1);
    expect(messages[0].content).toBe("valid");
  });

  it("restores interrupted assistant responses as retryable errors", () => {
    const messages = restoreSessionMessages([
      { role: "user", content: "question" },
      { role: "assistant", content: "partial response", tool_name: AI_ERROR_TOOL_NAME },
    ]);
    expect(messages[1]).toMatchObject({
      role: "assistant",
      content: "partial response",
      error: true,
    });
  });
});
