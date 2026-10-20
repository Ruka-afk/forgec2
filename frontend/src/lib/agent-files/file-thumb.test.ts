import { describe, expect, it } from "vitest";
import type { FileSearchHit } from "./file-search";
import { isThumbCandidate, parseFileThumbResult, thumbCandidateBatches } from "./file-thumb";

function hit(path: string, category: FileSearchHit["category"]): FileSearchHit {
  return {
    path,
    name: path.split(/[\\/]/).pop() || path,
    ext: path.split(".").pop() || "",
    category,
    size: 100,
    modified: "2026-10-20T10:00:00Z",
  };
}

describe("file thumbnails", () => {
  it("parses cached thumb metadata and requires a thumb id for ok items", () => {
    const parsed = parseFileThumbResult(
      JSON.stringify({
        items: [
          { path: "a.jpg", ok: true, thumb_id: "a".repeat(64), width: 320, height: 200, source_size: 4 },
          { path: "b.jpg", ok: false, error: "ffmpeg missing" },
          { path: "c.jpg", ok: true },
        ],
      }),
    );
    expect(parsed?.items).toHaveLength(3);
    expect(parsed?.items[0]).toMatchObject({ ok: true, width: 320, height: 200 });
    expect(parsed?.items[1]).toMatchObject({ ok: false, error: "ffmpeg missing" });
    expect(parsed?.items[2]?.ok).toBe(false);
  });

  it("rejects ACKs, errors and non-thumbnail JSON", () => {
    expect(parseFileThumbResult("")).toBeNull();
    expect(parseFileThumbResult("not json")).toBeNull();
    expect(parseFileThumbResult('{"success":true,"task_id":3}')).toBeNull();
  });

  it("batches only image/video hits, caps the total and groups by 12", () => {
    const hits = [
      hit("a.jpg", "image"),
      hit("b.txt", "document"),
      ...Array.from({ length: 30 }, (_, i) => hit(`v${i}.mp4`, "video")),
    ];
    const batches = thumbCandidateBatches(hits);
    expect(batches).toHaveLength(2);
    expect(batches[0]).toHaveLength(12);
    expect(batches[1]).toHaveLength(12);
    expect(batches.flat().some((item) => item.category === "document")).toBe(false);
  });

  it("identifies thumbnail-capable categories", () => {
    expect(isThumbCandidate("image")).toBe(true);
    expect(isThumbCandidate("video")).toBe(true);
    expect(isThumbCandidate("audio")).toBe(false);
  });
});
