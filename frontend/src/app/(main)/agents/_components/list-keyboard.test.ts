import { describe, expect, it } from "vitest";
import type { Beacon } from "./types";
import type { FlatBeaconRow } from "./groupBeaconsByHost";
import {
  hostKeyForFocus,
  listKeyAction,
  readExpandedHosts,
  writeExpandedHosts,
} from "./list-keyboard";

const beacon = (id: string): Beacon => ({ id, hostname: id, status: "online" });

describe("listKeyAction", () => {
  it("maps operator keys", () => {
    expect(listKeyAction("j")).toEqual({ type: "move", delta: 1 });
    expect(listKeyAction("k")).toEqual({ type: "move", delta: -1 });
    expect(listKeyAction("Enter")).toEqual({ type: "interact" });
    expect(listKeyAction("x")).toEqual({ type: "toggleSelect" });
    expect(listKeyAction(" ")).toEqual({ type: "toggleExpand" });
    expect(listKeyAction("/")).toEqual({ type: "focusSearch" });
    expect(listKeyAction("Escape", true)).toEqual({ type: "dismissMenu" });
    expect(listKeyAction("Escape", false)).toBeNull();
    expect(listKeyAction("a")).toBeNull();
  });
});

describe("hostKeyForFocus / expand persist", () => {
  it("finds the host row for the primary session", () => {
    const rows: FlatBeaconRow[] = [
      { kind: "host", group: { key: "h:box", hostname: "box", os: "windows", ip: "1.1.1.1", status: "online", last_seen: "", sessions: [beacon("a"), beacon("b")] } },
      { kind: "session", beacon: beacon("c"), child: false },
    ];
    expect(hostKeyForFocus(rows, "a")).toBe("h:box");
    expect(hostKeyForFocus(rows, "c")).toBeNull();
  });

  it("round-trips expanded host keys", () => {
    const mem: Record<string, string> = {};
    const storage = {
      getItem: (k: string) => mem[k] ?? null,
      setItem: (k: string, v: string) => { mem[k] = v; },
    };
    writeExpandedHosts(new Set(["h:box", "ip:10.0.0.1"]), storage);
    expect([...readExpandedHosts(storage)].sort()).toEqual(["h:box", "ip:10.0.0.1"]);
  });
});
