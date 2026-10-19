import { describe, expect, it } from "vitest";
import { groupBeaconsByHost, hostKey } from "./groupBeaconsByHost";
import type { Beacon } from "./types";

function beacon(partial: Partial<Beacon>): Beacon {
  return { id: "x", hostname: "box", status: "offline", ...partial };
}

describe("hostKey", () => {
  it("prefers hostname over ip", () => {
    expect(hostKey(beacon({ hostname: "DESKTOP-A", ip: "10.0.0.1" }))).toBe("h:desktop-a");
  });

  it("falls back to ip then id", () => {
    expect(hostKey(beacon({ hostname: "", ip: "10.0.0.2" }))).toBe("ip:10.0.0.2");
    expect(hostKey(beacon({ id: "abc", hostname: "", ip: "" }))).toBe("id:abc");
  });
});

describe("groupBeaconsByHost", () => {
  it("keeps a single session as its own host", () => {
    const groups = groupBeaconsByHost([beacon({ id: "1", hostname: "solo" })]);
    expect(groups).toHaveLength(1);
    expect(groups[0].sessions).toHaveLength(1);
  });

  it("merges same hostname and prefers online as primary", () => {
    const groups = groupBeaconsByHost([
      beacon({ id: "1", hostname: "WS01", status: "offline", last_seen: "2026-01-01T10:00:00Z", username: "old" }),
      beacon({ id: "2", hostname: "ws01", status: "online", last_seen: "2026-01-01T09:00:00Z", username: "admin" }),
    ]);
    expect(groups).toHaveLength(1);
    expect(groups[0].sessions.map((s) => s.id)).toEqual(["2", "1"]);
    expect(groups[0].status).toBe("online");
  });
});
