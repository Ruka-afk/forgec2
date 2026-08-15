import { describe, expect, it } from "vitest";
import {
  extractImmediateListing,
  filesLsTaskId,
  isFilesLsAck,
  parseLsListing,
} from "./ls-listing";

describe("isFilesLsAck", () => {
  it("treats a queue ACK as not a listing", () => {
    expect(isFilesLsAck({ success: true, task_id: 42 })).toBe(true);
    expect(isFilesLsAck({ success: true, task_id: 7, queued: true, kind: "ls_task" })).toBe(true);
    expect(isFilesLsAck({ files: [{ name: "a", is_dir: false, size: 1, mod_time: "" }] })).toBe(false);
    expect(extractImmediateListing({ success: true, task_id: 42 })).toBeNull();
    expect(filesLsTaskId({ task_id: 42 })).toBe(42);
  });
});

describe("parseLsListing", () => {
  it("parses the implant TSV table after a completed ls task", () => {
    const table = [
      "Type\tName\tSize\tModified",
      "--------------------------------------------------------------------------------",
      "DIR\tUsers\t-\t2026-01-02 10:00",
      "FILE\treadme.txt\t128\t2026-01-02 11:00",
    ].join("\n");
    expect(parseLsListing(table)).toEqual([
      { name: "Users", is_dir: true, size: 0, mod_time: "2026-01-02 10:00" },
      { name: "readme.txt", is_dir: false, size: 128, mod_time: "2026-01-02 11:00" },
    ]);
    expect(parseLsListing("")).toEqual([]);
  });
});
