import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { applyUrlState, readUrlState } from "./useUrlState";

const RANGES = ["24h", "7d", "30d"] as const;

describe("readUrlState", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", "/dashboard");
  });
  afterEach(() => {
    window.history.replaceState(null, "", "/dashboard");
  });

  it("returns a valid value from the query string", () => {
    window.history.replaceState(null, "", "/dashboard?range=7d");
    expect(readUrlState("range", "24h", RANGES)).toBe("7d");
  });

  it("falls back to initial for unknown or missing values", () => {
    window.history.replaceState(null, "", "/dashboard?range=100y");
    expect(readUrlState("range", "24h", RANGES)).toBe("24h");
    window.history.replaceState(null, "", "/dashboard");
    expect(readUrlState("range", "24h", RANGES)).toBe("24h");
  });

  it("keeps other unrelated query params intact", () => {
    window.history.replaceState(null, "", "/dashboard?listener_id=42&range=30d");
    expect(readUrlState("range", "24h", RANGES)).toBe("30d");
  });
});

describe("applyUrlState", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", "/dashboard");
  });
  afterEach(() => {
    window.history.replaceState(null, "", "/dashboard");
  });

  it("writes non-default values into the URL", () => {
    applyUrlState("range", "30d", "24h");
    expect(new URLSearchParams(window.location.search).get("range")).toBe("30d");
  });

  it("removes the param when the value returns to the default", () => {
    applyUrlState("range", "7d", "24h");
    applyUrlState("range", "24h", "24h");
    expect(new URLSearchParams(window.location.search).has("range")).toBe(false);
  });

  it("uses replaceState (no history entry pushed)", () => {
    const before = window.history.length;
    applyUrlState("status", "failed", "");
    expect(window.history.length).toBe(before);
  });
});