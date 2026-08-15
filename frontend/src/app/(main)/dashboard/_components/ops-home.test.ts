import { describe, expect, it } from "vitest";
import type { NormalizedAgent } from "@/types/agent";
import type { Task } from "@/types/task";
import type { LootData } from "@/types/loot";
import type { ListenerHealth } from "../../listeners/_components/listener-health";
import {
  flattenLoot,
  mergeAttention,
  pickUnhealthyListeners,
  splitSessions,
} from "./ops-home";

const agent = (over: Partial<NormalizedAgent>): NormalizedAgent => ({
  id: "1",
  hostname: "host",
  username: "u",
  ip: "10.0.0.1",
  os: "windows",
  status: "online",
  last_seen: "2026-08-13T12:00:00Z",
  listener_id: "1",
  tags: "",
  ...over,
});

describe("splitSessions", () => {
  it("keeps newest online and recently dropped", () => {
    const { online, dropped } = splitSessions([
      agent({ id: "a", status: "online", last_seen: "2026-08-13T10:00:00Z" }),
      agent({ id: "b", status: "online", last_seen: "2026-08-13T12:00:00Z" }),
      agent({ id: "c", status: "offline", last_seen: "2026-08-13T11:00:00Z" }),
      agent({ id: "d", status: "stale", last_seen: "2026-08-13T09:00:00Z" }),
    ], 1);
    expect(online.map((a) => a.id)).toEqual(["b"]);
    expect(dropped.map((a) => a.id)).toEqual(["c"]);
  });
});

describe("pickUnhealthyListeners", () => {
  it("returns burned before unstable by fail count", () => {
    const rows: Record<string, ListenerHealth> = {
      "1": { target: "1", status: "healthy", consecutive_fails: 0 },
      "2": { target: "2", status: "unstable", consecutive_fails: 2 },
      "3": { target: "3", status: "burned", consecutive_fails: 5 },
    };
    expect(pickUnhealthyListeners(rows).map((r) => r.target)).toEqual(["3", "2"]);
  });
});

describe("mergeAttention", () => {
  const task = (over: Partial<Task>): Task => ({
    id: 1,
    agent_id: "a",
    type: "shell",
    command: "whoami",
    status: "failed",
    result: "",
    error: "",
    created_by: "op",
    claimed_by: "",
    claimed_at: "",
    created_at: "2026-08-13T12:00:00Z",
    updated_at: "",
    ...over,
  });

  it("lists approvals first, then failed, then pending", () => {
    const merged = mergeAttention(
      [
        task({ id: 1, created_at: "2026-08-13T10:00:00Z" }),
        task({ id: 2, created_at: "2026-08-13T12:00:00Z" }),
      ],
      [task({ id: 3, status: "pending", created_at: "2026-08-13T13:00:00Z" })],
      [task({ id: 4, status: "pending_approval", created_at: "2026-08-13T09:00:00Z" })],
      10,
    );
    expect(merged.map((t) => t.id)).toEqual([4, 2, 1, 3]);
  });
});

describe("flattenLoot", () => {
  const loot: LootData = {
    screenshots: [{ id: "s1", agent_id: "a", filename: "desk.png", path: "", created_at: "2026-08-13T12:00:00Z" }],
    keylog_tasks: [{ id: "k1", agent_id: "a", result: "", error: "", status: "done", created_at: "2026-08-13T11:00:00Z" }],
    download_tasks: [{ id: "d1", agent_id: "b", command: "C:\\secret", result: "", status: "done", created_at: "2026-08-13T13:00:00Z" }],
  };

  it("sorts newest loot first", () => {
    expect(flattenLoot(loot).map((i) => i.kind)).toEqual(["download", "screenshot", "keylog"]);
  });
});
