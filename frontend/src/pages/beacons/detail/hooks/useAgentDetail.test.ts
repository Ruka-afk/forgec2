import { describe, it, expect } from "vitest";
import { mergeSnapshotWithPrev } from "./useAgentDetail";

interface Snap {
  tasks?: { id: number; status?: string; result?: string; error?: string }[];
  total_tasks?: number;
}

describe("mergeSnapshotWithPrev", () => {
  it("returns the snapshot untouched when there is no previous state", () => {
    const snap: Snap = { tasks: [{ id: 1, status: "pending" }] };
    expect(mergeSnapshotWithPrev(snap, null)).toBe(snap);
  });

  it("adopts WS-advanced status/result the snapshot has not caught up to", () => {
    const snap: Snap = { tasks: [{ id: 5, status: "running" }], total_tasks: 1 };
    const prev: Snap = { tasks: [{ id: 5, status: "completed", result: "whoami" }] };

    const next = mergeSnapshotWithPrev(snap, prev);

    expect(next.tasks?.[0]).toMatchObject({ id: 5, status: "completed", result: "whoami" });
    expect(next.total_tasks).toBe(1);
  });

  it("keeps snapshot fields the WS never advanced and ignores unknown ids", () => {
    const snap: Snap = {
      tasks: [
        { id: 5, status: "completed", result: "fresh" },
        { id: 6, status: "pending" },
      ],
    };
    const prev: Snap = {
      tasks: [
        { id: 5, status: "completed", result: "fresh" },
        { id: 999, status: "completed", result: "ghost" },
      ],
    };

    const next = mergeSnapshotWithPrev(snap, prev);

    // Nothing advanced: same reference back, no copies.
    expect(next).toBe(snap);
  });

  it("never overwrites snapshot truth with an older WS fallback", () => {
    const snap: Snap = { tasks: [{ id: 5, status: "completed", result: "FULL" }] };
    const prev: Snap = { tasks: [{ id: 5, status: "completed" }] };

    const next = mergeSnapshotWithPrev(snap, prev);

    expect(next).toBe(snap);
  });

  it("merges per-task: advances one row while leaving siblings alone", () => {
    const snap: Snap = {
      tasks: [
        { id: 1, status: "pending" },
        { id: 2, status: "pending" },
      ],
    };
    const prev: Snap = {
      tasks: [
        { id: 1, status: "failed", error: "timeout" },
        { id: 2, status: "pending" },
      ],
    };

    const next = mergeSnapshotWithPrev(snap, prev);

    expect(next.tasks?.[0]).toMatchObject({ id: 1, status: "failed", error: "timeout" });
    expect(next.tasks?.[1]).toEqual({ id: 2, status: "pending" });
  });
});
