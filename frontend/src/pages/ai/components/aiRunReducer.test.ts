import { describe, expect, it } from "vitest";
import { aiRunViewReducer, initialAIRunViewState } from "./aiRunReducer";

describe("aiRunViewReducer", () => {
  it("deduplicates replayed event ids and keeps cumulative text", () => {
    let state = aiRunViewReducer(initialAIRunViewState, { type: "started", runId: "run-1" });
    state = aiRunViewReducer(state, { type: "event", event: { id: "2", event: "text", data: "hello" } });
    state = aiRunViewReducer(state, { type: "event", event: { id: "2", event: "text", data: "duplicate" } });
    expect(state.text).toBe("hello");
    expect(state.lastEventId).toBe(2);
  });

  it("drops live reasoning when a run becomes durable terminal state", () => {
    let state = aiRunViewReducer(initialAIRunViewState, { type: "started", runId: "run-2", status: "running" });
    state = aiRunViewReducer(state, { type: "event", event: { id: "3", event: "reasoning", data: "transient" } });
    state = aiRunViewReducer(state, { type: "event", event: { id: "4", event: "done", data: "" } });
    expect(state.status).toBe("completed");
    expect(state.reasoning).toBe("");
  });
});
