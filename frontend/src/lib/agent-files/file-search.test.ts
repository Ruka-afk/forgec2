import { describe, expect, it } from "vitest";
import {
  buildFileSearchForm,
  emptyFileSearchResult,
  parseFileSearchResult,
} from "./file-search";

describe("parseFileSearchResult", () => {
  it("parses metadata hits and result counters", () => {
    const got = parseFileSearchResult(JSON.stringify({
      root: "C:\\Users\\a",
      patterns: ["*.jpg"],
      categories: ["image", "video"],
      hits: [
        {
          path: "C:\\Users\\a\\pic.JPG",
          name: "pic.JPG",
          ext: "jpg",
          category: "image",
          size: 1234,
          modified: "2026-10-20T10:00:00+08:00",
        },
      ],
      scanned: 42,
      errors: 2,
      truncated: true,
      truncated_reason: "max_results",
      elapsed_ms: 345,
    }));

    expect(got).not.toBeNull();
    expect(got?.root).toBe("C:\\Users\\a");
    expect(got?.hits).toEqual([
      {
        path: "C:\\Users\\a\\pic.JPG",
        name: "pic.JPG",
        ext: "jpg",
        category: "image",
        size: 1234,
        modified: "2026-10-20T10:00:00+08:00",
      },
    ]);
    expect(got?.scanned).toBe(42);
    expect(got?.errors).toBe(2);
    expect(got?.truncated).toBe(true);
    expect(got?.truncatedReason).toBe("max_results");
    expect(got?.elapsedMs).toBe(345);
  });

  it("repairs missing names/extensions and rejects malformed entries", () => {
    const got = parseFileSearchResult(JSON.stringify({
      hits: [
        { path: "/home/a/no-name.MP4", category: "not-real", size: -5 },
        { name: "missing-path.jpg" },
        { path: "/home/a/file", name: "file", category: "other", size: "12" },
      ],
    }));
    expect(got?.hits).toHaveLength(2);
    expect(got?.hits[0]).toMatchObject({
      path: "/home/a/no-name.MP4",
      name: "no-name.MP4",
      ext: "mp4",
      category: "other",
      size: 0,
    });
    expect(got?.hits[1]).toMatchObject({
      path: "/home/a/file",
      name: "file",
      ext: "",
      category: "other",
      size: 12,
    });
    expect(got?.truncated).toBe(false);
  });

  it("returns null for ACKs, error text, and non-search JSON", () => {
    expect(parseFileSearchResult("")).toBeNull();
    expect(parseFileSearchResult("not json")).toBeNull();
    expect(parseFileSearchResult(JSON.stringify({ success: true, task_id: 7 }))).toBeNull();
    expect(parseFileSearchResult(JSON.stringify({ root: 1, hits: "nope" }))).toBeNull();
  });
});

describe("buildFileSearchForm", () => {
  it("normalizes optional fields to the file API form contract", () => {
    expect(buildFileSearchForm({
      root: "C:\\Users\\a",
      pattern: "  ",
      categories: ["image", "video"],
      minSize: 1024,
      maxSize: 2048,
      modifiedAfter: "2026-01-01",
      includeHidden: true,
      maxDepth: 4,
    })).toEqual({
      path: "C:\\Users\\a",
      pattern: "*",
      categories: "image,video",
      min_size: "1024",
      max_size: "2048",
      modified_after: "2026-01-01",
      modified_before: "",
      include_hidden: "1",
      max_depth: "4",
      max_results: "2000",
      timeout_seconds: "20",
    });
  });

  it("keeps an all-category search empty and clamps negative sizes", () => {
    const form = buildFileSearchForm({
      root: "/tmp",
      pattern: "*.png",
      categories: [],
      minSize: -1,
      maxSize: Number.NaN,
    });
    expect(form.categories).toBe("");
    expect(form.min_size).toBe("0");
    expect(form.max_size).toBe("0");
    expect(form.include_hidden).toBe("0");
    expect(form.max_depth).toBe("8");
  });
});

describe("emptyFileSearchResult", () => {
  it("always exposes an empty, non-truncated hit list", () => {
    const empty = emptyFileSearchResult("/tmp");
    expect(empty).toEqual({
      root: "/tmp",
      hits: [],
      scanned: 0,
      errors: 0,
      truncated: false,
      truncatedReason: "",
      elapsedMs: 0,
    });
  });
});
