import { describe, expect, it } from "vitest";
import {
  diffChangeLines,
  diffResults,
  previousComparableTask,
  resultLooksComparable,
} from "./result-diff";

describe("resultLooksComparable", () => {
  it("rejects empty, image, and base64 blobs", () => {
    expect(resultLooksComparable("")).toBe(false);
    expect(resultLooksComparable("data:image/png;base64,aaaa")).toBe(false);
    expect(resultLooksComparable("a".repeat(90))).toBe(true);
    expect(resultLooksComparable(`${"A".repeat(90)}==`)).toBe(false);
  });
});

describe("previousComparableTask", () => {
  const row = (over: Record<string, unknown>) => ({
    id: 1,
    type: "ps",
    command: "ps",
    status: "completed",
    result: "a.exe",
    ...over,
  });

  it("prefers the newest earlier task of the same type and command", () => {
    const prev = previousComparableTask(
      [
        row({ id: 5, result: "now" }),
        row({ id: 4, command: "whoami", result: "alice" }),
        row({ id: 3, result: "old" }),
        row({ id: 2, status: "failed", result: "nope" }),
      ],
      row({ id: 5 }),
    );
    expect(prev?.id).toBe(3);
  });
});

describe("diffResults", () => {
  it("marks added and removed lines in sequence", () => {
    const diff = diffResults("one\ntwo\n", "one\nthree\n");
    expect(diff.mode).toBe("sequence");
    expect(diff.added).toBe(1);
    expect(diff.removed).toBe(1);
    expect(diffChangeLines(diff)).toEqual([
      { kind: "del", text: "two" },
      { kind: "add", text: "three" },
    ]);
  });

  it("reports no changes when the text matches", () => {
    const diff = diffResults("same\n", "same\n");
    expect(diff.added).toBe(0);
    expect(diff.removed).toBe(0);
  });

  it("falls back to unique lines when the output is too long for sequential LCS", () => {
    const before = Array.from({ length: 401 }, (_, i) => `L${i}`).join("\n");
    const after = `${before}\nL401`;
    const diff = diffResults(before, after);
    expect(diff.mode).toBe("unique");
    expect(diff.added).toBe(1);
    expect(diff.removed).toBe(0);
  });
});
