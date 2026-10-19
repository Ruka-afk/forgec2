import { describe, expect, it } from "vitest";
import { mergePolledWithLive, unifiedFromWS, upsertLiveEvent } from "./ws-to-event";
import type { UnifiedEvent } from "./types";

const ev = (over: Partial<UnifiedEvent>): UnifiedEvent => ({
  id: "a",
  at: "2026-08-14T10:00:00Z",
  source: "timeline",
  kind: "agent_online",
  title: "box",
  detail: "",
  ...over,
});

describe("unifiedFromWS", () => {
  it("maps agent and task frames", () => {
    const online = unifiedFromWS({ type: "agent_online", agent_id: "a1", hostname: "DC01", ip: "10.0.0.1" }, "t0");
    expect(online?.source).toBe("timeline");
    expect(online?.id).toBe("ws-agent_online-a1");
    expect(online?.title).toBe("DC01");

    const task = unifiedFromWS({ type: "task_update", task_id: 9, task_type: "shell", command: "whoami", status: "failed", agent_id: "a1" }, "t1");
    expect(task?.id).toBe("task-9");
    expect(task?.source).toBe("task");
    expect(task?.status).toBe("failed");
  });

  it("ignores noise frames", () => {
    expect(unifiedFromWS({ type: "pong" })).toBeNull();
    expect(unifiedFromWS({ type: "sync" })).toBeNull();
  });
});

describe("upsert + merge", () => {
  it("lets live frames replace the polled row with the same id", () => {
    const live = upsertLiveEvent([], ev({ id: "task-9", status: "failed", at: "2026-08-14T12:00:00Z" }));
    const merged = mergePolledWithLive(
      [ev({ id: "task-9", status: "pending", at: "2026-08-14T11:00:00Z" }), ev({ id: "tl-1", at: "2026-08-14T09:00:00Z" })],
      live,
    );
    expect(merged[0].id).toBe("task-9");
    expect(merged[0].status).toBe("failed");
  });
});
