import { describe, expect, it } from "vitest";
import {
  deleteCopyKind,
  deleteLooksConfirmed,
  EXFIL_CAP,
  EXFIL_CHUNK,
  exfilBasename,
  fileReadPreview,
  fileTaskId,
  isFileTaskAck,
  looksLikeFileTaskAckJson,
  parseFindResult,
  pullPlan,
  transferChunkCount,
  transferPercent,
  transferProgressAt,
} from "./file-task";

describe("isFileTaskAck", () => {
  it("treats dispatchTask JSON as a queue ACK, not file content", () => {
    expect(isFileTaskAck({ success: true, task_id: 9 })).toBe(true);
    expect(fileTaskId({ success: true, task_id: 9 })).toBe(9);
    expect(isFileTaskAck({ content: "hello world", task_id: 1 })).toBe(false);
    expect(looksLikeFileTaskAckJson(JSON.stringify({ success: true, task_id: 3 }))).toBe(true);
    expect(looksLikeFileTaskAckJson("hello world")).toBe(false);
  });
});

describe("fileReadPreview / delete copy", () => {
  it("does not preview ACK JSON and does not treat queued delete as confirmed", () => {
    expect(fileReadPreview(JSON.stringify({ success: true, task_id: 4 }), false).content).toBe("");
    expect(fileReadPreview("secret.txt body", false)).toEqual({ content: "secret.txt body", isImage: false });
    expect(deleteCopyKind(true)).toBe("queued");
    expect(deleteLooksConfirmed("queued")).toBe(false);
    expect(deleteLooksConfirmed("deleted")).toBe(true);
  });
});

describe("exfil helpers", () => {
  it("takes the basename from mixed separators", () => {
    expect(exfilBasename("C:\\Users\\admin\\secret.txt")).toBe("secret.txt");
    expect(exfilBasename("/etc/shadow")).toBe("shadow");
  });

  it("caps a pull at 50 MiB and defaults unknown size to one chunk", () => {
    expect(pullPlan(0)).toEqual({ total: EXFIL_CHUNK, chunk: EXFIL_CHUNK, partial: true });
    expect(pullPlan(128)).toEqual({ total: 128, chunk: EXFIL_CHUNK, partial: false });
    expect(pullPlan(EXFIL_CAP + 1)).toEqual({ total: EXFIL_CAP, chunk: EXFIL_CHUNK, partial: true });
  });

  it("reports chunk index and percent against the plan", () => {
    const plan = { total: EXFIL_CHUNK * 2, chunk: EXFIL_CHUNK, partial: false };
    expect(transferChunkCount(plan.total, plan.chunk)).toBe(2);
    expect(transferProgressAt(0, plan)).toMatchObject({ chunkIndex: 0, chunkCount: 2, offset: 0 });
    expect(transferProgressAt(EXFIL_CHUNK, plan)).toMatchObject({ chunkIndex: 1, chunkCount: 2 });
    expect(transferPercent({ offset: EXFIL_CHUNK, total: EXFIL_CHUNK * 2 })).toBe(50);
  });

  it("parses find TSV and ignores a queue ACK", () => {
    expect(parseFindResult("C:\\a.txt\t12\t2026-01-01 00:00\nC:\\b.log\t3\t2026-01-01 00:01")).toEqual([
      "C:\\a.txt",
      "C:\\b.log",
    ]);
    expect(parseFindResult(JSON.stringify({ success: true, task_id: 9 }))).toEqual([]);
    expect(parseFindResult("# file_hunt root=/tmp\npath\tsize\tmtime\tstatus\n/tmp/a.txt\t1\t2026-01-01 00:00\tlisted\n=== downloaded ===\n=== file path=/tmp/a.txt ===\n")).toEqual([
      "/tmp/a.txt",
    ]);
  });
});
