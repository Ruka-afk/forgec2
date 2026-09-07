import { describe, it, expect } from "vitest";
import { emptyLootData, lootDeleteId, normalizeLootData } from "./types";
import { canDownloadLootExfil, lootExfilBytes, lootExfilFilename } from "./loot-exfil";

describe("normalizeLootData", () => {
  it("returns empty for null", () => {
    expect(normalizeLootData(null)).toEqual(emptyLootData());
  });

  it("maps screenshots and snake_case keylogs/downloads", () => {
    const r = normalizeLootData({
      screenshots: [{ id: "s1", agent_id: "a", filename: "x.png", path: "a/x.png" }],
      keylogs: [{ id: "k1" }],
      downloads: [{ id: "d1" }],
    });
    expect(r.screenshots).toHaveLength(1);
    expect(r.keylog_tasks).toHaveLength(1);
    expect(r.download_tasks).toHaveLength(1);
  });

  it("accepts PascalCase screenshot DTOs from dual-use /loot", () => {
    const r = normalizeLootData({
      Screenshots: [{ AgentID: "host1", Filename: "shot.png", Path: "host1/shot.png" }],
    });
    expect(r.screenshots[0]).toMatchObject({
      id: "screenshot:host1:shot.png",
      agent_id: "host1",
      filename: "shot.png",
      path: "host1/shot.png",
    });
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

describe("lootDeleteId", () => {
  it("prefixes task ids for bulk delete", () => {
    expect(lootDeleteId("download", { id: "12" })).toBe("download:12");
    expect(lootDeleteId("download", { id: "download:12" })).toBe("download:12");
    expect(lootDeleteId("screenshot", { id: "x", agent_id: "a", filename: "f.png" })).toBe("screenshot:a:f.png");
  });
});

describe("lootExfilFilename", () => {
  it("takes the basename from a remote path and ignores URL fetches", () => {
    expect(lootExfilFilename({ command: "C:\\Users\\admin\\secret.txt" })).toBe("secret.txt");
    expect(lootExfilFilename({ command: "/etc/shadow" })).toBe("shadow");
    expect(lootExfilFilename({ command: "https://evil/stage.bin" })).toBeNull();
    expect(lootExfilFilename({ command: "", result: "File chunk saved: loot.bin offset 0 (12 bytes)" })).toBe("loot.bin");
  });
});

describe("lootExfilBytes / canDownloadLootExfil", () => {
  it("parses the chunk size and only offers a blob for completed pulls", () => {
    expect(lootExfilBytes("File chunk saved: loot.bin offset 0 (12 bytes)")).toBe(12);
    expect(canDownloadLootExfil({ status: "completed", command: "C:\\secret.txt" })).toBe(true);
    expect(canDownloadLootExfil({ status: "pending", command: "C:\\secret.txt" })).toBe(false);
    expect(canDownloadLootExfil({ status: "completed", command: "https://evil/stage.bin" })).toBe(false);
  });
});
