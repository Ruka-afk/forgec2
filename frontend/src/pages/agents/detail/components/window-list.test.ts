import { describe, expect, it } from "vitest";
import { parseWindowList } from "./window-list";

describe("parseWindowList", () => {
  it("parses HWND rows and skips header/trailer noise", () => {
    const raw = [
      "HWND\tPID\tTITLE",
      "131074\t1234\tUntitled - Notepad",
      "131075\t5678\tSettings",
      "# windows=2",
    ].join("\n");
    expect(parseWindowList(raw)).toEqual([
      { hwnd: "131074", pid: "1234", title: "Untitled - Notepad" },
      { hwnd: "131075", pid: "5678", title: "Settings" },
    ]);
  });

  it("skips non-numeric rows and returns empty for blank payloads", () => {
    const raw = ["HWND\tPID\tTITLE", "oops\tnot-a-row", ""].join("\n");
    expect(parseWindowList(raw)).toEqual([]);
    expect(parseWindowList("")).toEqual([]);
  });
});
