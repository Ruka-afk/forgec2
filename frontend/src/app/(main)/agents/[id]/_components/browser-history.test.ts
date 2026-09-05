import { describe, expect, it } from "vitest";
import { parseBrowserHistory } from "./browser-history";

describe("parseBrowserHistory", () => {
  it("parses chromium rows under section headers", () => {
    const raw = [
      "=== browser history (URLs/titles only, capped) ===",
      "=== Chrome (C:\\Users\\a\\History) ===",
      "2026-01-02T03:04:05Z\t12\thttps://example.com/\tExample",
      "2026-01-01T00:00:00Z\t1\thttps://x.test/\t",
      "# rows=2",
    ].join("\n");
    const rows = parseBrowserHistory(raw);
    expect(rows).toEqual([
      { browser: "Chrome", time: "2026-01-02T03:04:05Z", visits: "12", url: "https://example.com/", title: "Example" },
      { browser: "Chrome", time: "2026-01-01T00:00:00Z", visits: "1", url: "https://x.test/", title: "" },
    ]);
  });

  it("parses safari two-column rows and skips noise", () => {
    const raw = [
      "=== Safari (/tmp/History.db) ===",
      "2026-02-03T04:05:06Z\thttps://safari.test/a",
      "query: locked",
      "(not found)",
      "browser_history: unknown browser filter (x)",
    ].join("\n");
    expect(parseBrowserHistory(raw)).toEqual([
      { browser: "Safari", time: "2026-02-03T04:05:06Z", visits: "", url: "https://safari.test/a", title: "" },
    ]);
  });

  it("returns empty for blank or ack payloads", () => {
    expect(parseBrowserHistory("")).toEqual([]);
    expect(parseBrowserHistory(JSON.stringify({ success: true, task_id: 7 }))).toEqual([]);
  });
});
