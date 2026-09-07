import { describe, expect, it } from "vitest";
import { artifactDownloadable, artifactsForListener, normalizeBuildLog } from "./listener-artifacts";

describe("normalizeBuildLog", () => {
  it("maps snake_case and PascalCase build logs", () => {
    expect(normalizeBuildLog({
      id: 4,
      filename: "beacon.exe",
      format: "exe",
      platform: "windows",
      status: "success",
      created_at: "2026-08-14T12:00:00Z",
      listener_id: 3,
    })).toMatchObject({ id: "4", filename: "beacon.exe", listener_id: "3" });
    expect(normalizeBuildLog({ ID: 8, Filename: "a.bin", ListenerID: 2 })?.listener_id).toBe("2");
  });
});

describe("artifactsForListener", () => {
  it("keeps rows for this listener and drops other listeners", () => {
    const rows = artifactsForListener(
      [
        { id: 1, listener_id: 3, filename: "a.exe", status: "success" },
        { id: 2, listener_id: 9, filename: "b.exe", status: "success" },
      ],
      "3",
    );
    expect(rows.map((r) => r.id)).toEqual(["1"]);
    expect(artifactDownloadable("success")).toBe(true);
    expect(artifactDownloadable("failed")).toBe(false);
  });
});
