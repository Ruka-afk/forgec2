import { describe, it, expect } from "vitest";
import { aggregateTaskStats, computeDateRange, severityColor } from "./types";

describe("report helpers", () => {
  it("aggregateTaskStats groups raw task rows by type", () => {
    const stats = aggregateTaskStats([
      { type: "shell", status: "completed" },
      { type: "shell", status: "failed" },
      { type: "shell", status: "pending" },
      { type: "upload", status: "completed" },
      {},
    ]);
    expect(stats).toEqual([
      { type: "shell", total: 3, success: 1, failed: 1, success_rate: 33 },
      { type: "upload", total: 1, success: 1, failed: 0, success_rate: 100 },
      { type: "unknown", total: 1, success: 0, failed: 0, success_rate: 0 },
    ]);
  });

  it("aggregateTaskStats tolerates an empty list", () => {
    expect(aggregateTaskStats([])).toEqual([]);
  });

  it("severityColor maps known levels", () => {
    expect(severityColor("critical")).toMatch(/destructive/);
    expect(severityColor("high")).toMatch(/warning/);
    expect(severityColor("medium")).toMatch(/warning/);
    expect(severityColor("unknown")).toMatch(/muted/);
  });

  it("computeDateRange uses custom bounds", () => {
    expect(computeDateRange("custom", "2024-01-01", "2024-01-31")).toEqual({
      start: "2024-01-01",
      end: "2024-01-31",
    });
  });

  it("computeDateRange presets produce ISO dates", () => {
    const r = computeDateRange("7d", "", "");
    expect(r.start).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(r.end).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(new Date(r.end).getTime()).toBeGreaterThanOrEqual(new Date(r.start).getTime());
  });
});
