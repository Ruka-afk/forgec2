import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { __testDispatchWS } from "./wsContext";
import { subscribeTyped, isWSEvent } from "./typed-ws";

describe("typed-ws", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("delivers only matching registered events", () => {
    const cb = vi.fn();
    const unsub = subscribeTyped("task_update", cb);
    __testDispatchWS({ type: "task_update", task_id: 7, status: "running" });
    __testDispatchWS({ type: "task_output", task_id: 7, chunk: "x" });
    __testDispatchWS({ type: "notification", message: "hi" });
    unsub();
    expect(cb).toHaveBeenCalledTimes(1);
    expect(cb.mock.calls[0][0].task_id).toBe(7);
  });

  it("supports multi-type subscriptions", () => {
    const cb = vi.fn();
    const unsub = subscribeTyped(["agent_online", "agent_offline"], cb);
    __testDispatchWS({ type: "agent_online", agent_id: "a1", hostname: "host" });
    __testDispatchWS({ type: "task_update", task_id: 1, status: "done" });
    unsub();
    expect(cb).toHaveBeenCalledTimes(1);
    expect(cb.mock.calls[0][0].hostname).toBe("host");
  });

  it("ignores frames without a string type", () => {
    const cb = vi.fn();
    subscribeTyped("task_update", cb);
    __testDispatchWS({ task_id: 1 } as never);
    expect(cb).not.toHaveBeenCalled();
  });

  it("unsubscribe stops delivery", () => {
    const cb = vi.fn();
    const unsub = subscribeTyped("task_update", cb);
    unsub();
    __testDispatchWS({ type: "task_update", task_id: 1, status: "running" });
    expect(cb).not.toHaveBeenCalled();
  });

  it("isWSEvent narrows the message type", () => {
    const msg = { type: "system_alert", message: "alert!", alert_type: "critical" };
    expect(isWSEvent(msg, "system_alert")).toBe(true);
    expect(isWSEvent(msg, "credential_found")).toBe(false);
  });
});