import { describe, it, expect } from "vitest";
import { computeDateRange, severityColor } from "./types";

describe("report helpers", () => {
  it("severityColor maps known levels", () => {
    expect(severityColor("critical")).toMatch(/red/);
    expect(severityColor("high")).toMatch(/orange/);
    expect(severityColor("medium")).toMatch(/yellow/);
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
