import { describe, expect, it } from "vitest";
import { parsePsRows } from "./process-snapshot";

describe("parsePsRows", () => {
  it("parses Windows Format-Table output, skipping headers", () => {
    const raw = [
      "ProcessId Name         WorkingSetMB",
      "--------- ----         ------------",
      "1234      chrome.exe   512.5",
      "56        svchost.exe  100",
    ].join("\n");
    expect(parsePsRows(raw)).toEqual([
      { pid: "1234", name: "chrome.exe", raw: "1234      chrome.exe   512.5" },
      { pid: "56", name: "svchost.exe", raw: "56        svchost.exe  100" },
    ]);
  });

  it("parses ps aux output, skipping the header", () => {
    const raw = [
      "USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND",
      "root       123  0.1  0.2 123456  7890 ?        Ss   10:00   0:01 /usr/bin/sshd",
    ].join("\n");
    const rows = parsePsRows(raw);
    expect(rows).toHaveLength(1);
    expect(rows[0].pid).toBe("123");
    expect(rows[0].name).toContain("sshd");
  });

  it("returns empty for blank text and caps rows", () => {
    expect(parsePsRows("")).toEqual([]);
    const many = Array.from({ length: 300 }, (_, i) => `${1000 + i} proc${i}.exe`).join("\n");
    expect(parsePsRows(many)).toHaveLength(200);
  });
});
