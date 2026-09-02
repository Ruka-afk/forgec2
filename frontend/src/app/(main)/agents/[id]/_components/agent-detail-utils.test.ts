import { describe, expect, it } from "vitest";
import { agentDetailHref } from "./agent-detail-utils";

describe("agentDetailHref", () => {
  it("builds the canonical agent session path", () => {
    expect(agentDetailHref("eac1558c-3a53-41e1-9498-6e6ded52d6ae"))
      .toBe("/agents/eac1558c-3a53-41e1-9498-6e6ded52d6ae");
  });
});
