import { describe, expect, it } from "vitest";
import {
  describeProcessSnapshot,
  isLiveProcessTree,
  PROCESS_SNAPSHOT_KIND,
} from "./process-snapshot";

describe("describeProcessSnapshot", () => {
  it("treats the shipped process-tree envelope as a last ps snapshot, not a live tree", () => {
    const snap = describeProcessSnapshot({
      processes: "pid 1 explorer.exe",
      source: "ps",
      live: false,
      kind: PROCESS_SNAPSHOT_KIND,
      alias_of: "ps",
    });
    expect(snap.live).toBe(false);
    expect(snap.source).toBe("ps");
    expect(snap.kind).toBe(PROCESS_SNAPSHOT_KIND);
    expect(snap.text).toBe("pid 1 explorer.exe");
    expect(isLiveProcessTree({ live: false, kind: PROCESS_SNAPSHOT_KIND })).toBe(false);
    expect(isLiveProcessTree({ live: true, kind: PROCESS_SNAPSHOT_KIND })).toBe(false);
  });
});
