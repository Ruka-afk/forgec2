import { describe, it, expect } from "vitest";
import { emptyLootData, normalizeLootData } from "./types";

describe("normalizeLootData", () => {
  it("returns empty for null", () => {
    expect(normalizeLootData(null)).toEqual(emptyLootData());
  });

  it("maps screenshots and snake_case keylogs/downloads", () => {
    const r = normalizeLootData({
      screenshots: [{ id: "s1" }],
      keylogs: [{ id: "k1" }],
      downloads: [{ id: "d1" }],
    });
    expect(r.screenshots).toHaveLength(1);
    expect(r.keylog_tasks).toHaveLength(1);
    expect(r.download_tasks).toHaveLength(1);
  });

  it("prefers keylog_tasks / download_tasks", () => {
    const r = normalizeLootData({
      keylog_tasks: [{ id: "a" }],
      download_tasks: [{ id: "b" }],
      keylogs: [{ id: "ignored" }],
    });
    expect(r.keylog_tasks[0]).toMatchObject({ id: "a" });
    expect(r.download_tasks[0]).toMatchObject({ id: "b" });
  });
});
