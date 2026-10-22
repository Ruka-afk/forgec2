import { describe, expect, it } from "vitest";
import {
  buildProcessTree,
  filterProcessTree,
  flattenProcessTree,
  isProtectedPid,
  parsePsRows,
  parseProcessSnapshot,
  processProtection,
  sortProcessTree,
} from "./process-snapshot";

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

describe("parseProcessSnapshot", () => {
  it("parses the Go process-tree TSV without showing USER as the process name", () => {
    const raw = [
      "PID\tPPID\tUSER\tNAME",
      "1\t0\troot\tsystemd",
      "  42\t1\troot\tsshd",
      "    77\t42\talice\tbash",
      "88\t1\t\tWindowsWorker.exe",
    ].join("\n");

    const snapshot = parseProcessSnapshot(raw);

    expect(snapshot.format).toBe("tree");
    expect(snapshot.rows.map((row) => row.name)).toEqual([
      "systemd",
      "sshd",
      "bash",
      "WindowsWorker.exe",
    ]);
    expect(snapshot.rows[2]).toMatchObject({ pid: "77", ppid: "42", user: "alice", depth: 2 });
    expect(snapshot.rows[3].user).toBeUndefined();

    const sshd = snapshot.roots[0].children[0];
    expect(sshd.name).toBe("sshd");
    expect(sshd.children[0].name).toBe("bash");
    expect(sshd.children[0].depth).toBe(2);
  });

  it("parses headerless tree and C TSV rows from older agents", () => {
    const tree = parseProcessSnapshot([
      "1\t0\troot\tsystemd",
      "  42\t1\troot\tsshd",
    ].join("\n"));
    expect(tree.format).toBe("tree");
    expect(tree.rows[1]).toMatchObject({ pid: "42", ppid: "1", user: "root", name: "sshd" });

    const c = parseProcessSnapshot([
      "4\t0\t12\t-22\tSystem",
      "1234\t4\t8\t15360\tchrome.exe",
    ].join("\n"));
    expect(c.format).toBe("c-ps");
    expect(c.rows[1]).toMatchObject({
      pid: "1234",
      ppid: "4",
      threads: 8,
      memoryKb: 15360,
      name: "chrome.exe",
    });
  });

  it("parses tasklist tables without truncating multi-word image names", () => {
    const raw = [
      "Image Name                     PID Session Name        Session#    Mem Usage",
      "========================= ======== ================ =========== ============",
      "System Idle Process              0 Services                   0          8 K",
      "chrome.exe                    1234 Console                    1     153600 K",
    ].join("\n");

    const snapshot = parseProcessSnapshot(raw);

    expect(snapshot.format).toBe("powershell");
    expect(snapshot.rows[0]).toMatchObject({ pid: "0", name: "System Idle Process" });
    expect(snapshot.rows[1]).toMatchObject({ pid: "1234", name: "chrome.exe" });
  });

  it("parses C implant ps TSV with threads and memory", () => {
    const raw = [
      "PID\tPPID\tTHREADS\tMEM_KB\tNAME",
      "4\t0\t12\t-22\tSystem",
      "1234\t4\t8\t15360\tchrome.exe",
    ].join("\n");

    const snapshot = parseProcessSnapshot(raw);

    expect(snapshot.format).toBe("c-ps");
    expect(snapshot.rows[1]).toMatchObject({
      pid: "1234",
      ppid: "4",
      threads: 8,
      memoryKb: 15360,
      name: "chrome.exe",
    });
  });

  it("parses PowerShell Format-Table columns and command lines", () => {
    const raw = [
      "ProcessId Name           CommandLine                         WorkingSetMB",
      "--------- ----           -----------                         ------------",
      "1234      chrome.exe     \"C:\\Chrome\\chrome.exe\" --type=      512.5",
      "88        svchost.exe    C:\\Windows\\svchost.exe -k netsvcs   100",
    ].join("\n");

    const snapshot = parseProcessSnapshot(raw);

    expect(snapshot.format).toBe("ps");
    expect(snapshot.rows[0]).toMatchObject({
      pid: "1234",
      name: "chrome.exe",
      command: "\"C:\\Chrome\\chrome.exe\" --type=",
      memoryKb: 524800,
    });
    expect(snapshot.rows[1].memoryKb).toBe(102400);
  });

  it("uses PPIDs to attach orphans and render cycle members safely", () => {
    const rows = parseProcessSnapshot([
      "PID\tPPID\tUSER\tNAME",
      "1\t0\troot\tinit",
      "30\t99\troot\torphan-a",
      "40\t41\troot\tcycle-a",
      "41\t40\troot\tcycle-b",
    ].join("\n")).rows;

    const roots = buildProcessTree(rows);
    expect(roots.map((node) => node.pid)).toEqual(["1", "30", "40"]);
    expect(roots[1].depth).toBe(0);
    expect(roots[2].children[0].pid).toBe("41");
    expect(roots[2].children[0].children).toEqual([]);
  });

  it("filters to matching descendants and their ancestors", () => {
    const snapshot = parseProcessSnapshot([
      "PID\tPPID\tUSER\tNAME",
      "1\t0\troot\tsystemd",
      "42\t1\troot\tsshd",
      "77\t42\talice\tbash",
      "90\t1\troot\tcron",
    ].join("\n"));

    const filtered = filterProcessTree(snapshot.roots, "bash");
    expect(flattenProcessTree(filtered).map((node) => node.name)).toEqual([
      "systemd",
      "sshd",
      "bash",
    ]);
  });

  it("sorts children recursively without losing the hierarchy", () => {
    const snapshot = parseProcessSnapshot([
      "PID\tPPID\tUSER\tNAME",
      "1\t0\troot\troot-proc",
      "10\t1\troot\tzeta",
      "11\t1\troot\talpha",
    ].join("\n"));

    const sorted = sortProcessTree(snapshot.roots, "name", "asc");
    expect(sorted[0].children.map((node) => node.name)).toEqual(["alpha", "zeta"]);
  });

  it("protects core and well-known system processes from termination", () => {
    expect(processProtection({ pid: "0", name: "System Idle Process" })).toBe("core");
    expect(processProtection({ pid: "500", name: "lsass.exe" })).toBe("critical");
    expect(processProtection({ pid: "1234", name: "notepad.exe" })).toBeNull();
    expect(isProtectedPid("4")).toBe(true);
    expect(isProtectedPid("1234")).toBe(false);
  });
});
