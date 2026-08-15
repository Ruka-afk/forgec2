import { describe, expect, it } from "vitest";
import type { Beacon } from "./types";
import {
  clampDockHeight,
  DOCK_HEIGHT_DEFAULT,
  DOCK_HEIGHT_MAX,
  DOCK_HEIGHT_MIN,
  isEditableTarget,
  nextSessionId,
  readInteractPrefs,
  sessionIdsFromRows,
  tabFromDigit,
  writeInteractPrefs,
} from "./interact-workspace";
import type { FlatBeaconRow } from "./groupBeaconsByHost";

const beacon = (id: string): Beacon => ({ id, hostname: id, status: "online" });

describe("clampDockHeight", () => {
  it("clamps to the operator range", () => {
    expect(clampDockHeight(50)).toBe(DOCK_HEIGHT_MIN);
    expect(clampDockHeight(900)).toBe(DOCK_HEIGHT_MAX);
    expect(clampDockHeight(Number.NaN)).toBe(DOCK_HEIGHT_DEFAULT);
  });
});

describe("sessionIdsFromRows", () => {
  it("dedupes host primary with expanded children", () => {
    const rows: FlatBeaconRow[] = [
      { kind: "host", group: { key: "h:box", hostname: "box", os: "windows", ip: "1.1.1.1", status: "online", last_seen: "", sessions: [beacon("a"), beacon("b")] } },
      { kind: "session", beacon: beacon("a"), child: true },
      { kind: "session", beacon: beacon("b"), child: true },
      { kind: "session", beacon: beacon("c"), child: false },
    ];
    expect(sessionIdsFromRows(rows)).toEqual(["a", "b", "c"]);
  });
});

describe("nextSessionId", () => {
  it("wraps around the visible session list", () => {
    expect(nextSessionId(["a", "b", "c"], "c", 1)).toBe("a");
    expect(nextSessionId(["a", "b", "c"], "a", -1)).toBe("c");
    expect(nextSessionId(["a", "b"], null, 1)).toBe("a");
    expect(nextSessionId([], "x", 1)).toBeNull();
  });
});

describe("tabFromDigit / prefs", () => {
  it("maps 1-3 to dock tabs", () => {
    expect(tabFromDigit("1")).toBe("shell");
    expect(tabFromDigit("2")).toBe("files");
    expect(tabFromDigit("3")).toBe("tasks");
    expect(tabFromDigit("4")).toBeNull();
  });

  it("only restores an agent when pinned", () => {
    const mem: Record<string, string> = {};
    const storage = {
      getItem: (k: string) => mem[k] ?? null,
      setItem: (k: string, v: string) => { mem[k] = v; },
    };
    writeInteractPrefs({ agentId: "abc", height: 400, tab: "files", pinned: false }, storage);
    expect(readInteractPrefs(storage).agentId).toBeNull();
    writeInteractPrefs({ agentId: "abc", height: 400, tab: "files", pinned: true }, storage);
    const prefs = readInteractPrefs(storage);
    expect(prefs.agentId).toBe("abc");
    expect(prefs.tab).toBe("files");
    expect(prefs.height).toBe(400);
  });
});

describe("isEditableTarget", () => {
  it("treats inputs as typing surfaces", () => {
    const input = document.createElement("input");
    expect(isEditableTarget(input)).toBe(true);
    expect(isEditableTarget(document.createElement("div"))).toBe(false);
  });
});
