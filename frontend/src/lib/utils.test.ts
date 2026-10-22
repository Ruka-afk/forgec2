import { describe, it, expect } from "vitest";
import { cn, formatTime, isZeroTime, timeAgo } from "./utils";

describe("cn", () => {
  it("keeps a text colour next to a custom font size", () => {
    // Regression: tailwind-merge did not know the dense-console sizes and
    // classified `text-compact` as a colour, deleting `text-primary-foreground`
    // from default buttons (dark text on a dark background).
    expect(cn("bg-primary text-primary-foreground text-compact")).toContain("text-primary-foreground");
    expect(cn("text-compact text-primary-foreground")).toContain("text-primary-foreground");
  });

  it("still lets a later colour override an earlier one", () => {
    expect(cn("text-muted-foreground text-destructive-ink")).toBe("text-destructive-ink");
  });

  it("still lets a later size override an earlier one", () => {
    expect(cn("text-xs text-compact")).toBe("text-compact");
  });
});

describe("isZeroTime", () => {
  it("treats Go's zero time as a placeholder", () => {
    expect(isZeroTime("0001-01-01T00:00:00Z")).toBe(true);
    expect(isZeroTime("")).toBe(true);
    expect(isZeroTime(null)).toBe(true);
    expect(isZeroTime(undefined)).toBe(true);
    expect(isZeroTime("not-a-date")).toBe(true);
  });

  it("keeps real timestamps", () => {
    expect(isZeroTime("2026-10-21T22:24:25.18+08:00")).toBe(false);
  });
});

describe("timeAgo", () => {
  it("does not turn the zero time into a huge age", () => {
    expect(timeAgo("0001-01-01T00:00:00Z", (k) => (k === "time.ago.unknown" ? "unknown" : k))).toBe("unknown");
  });

  it("still reports a real timestamp", () => {
    const twoHoursAgo = new Date(Date.now() - 2 * 3600 * 1000).toISOString();
    expect(timeAgo(twoHoursAgo, (k, p) => (k === "time.ago.hours" ? `${p?.n}h ago` : k))).toBe("2h ago");
  });
});

describe("formatTime", () => {
  it("renders an em dash for missing values", () => {
    expect(formatTime(null)).toBe("—");
    expect(formatTime("")).toBe("—");
  });
});
