import { describe, it, expect } from "vitest";
import { computeVirtualRange, VIRTUAL_THRESHOLD } from "./virtual";

describe("computeVirtualRange", () => {
  it("returns empty range for zero count", () => {
    expect(computeVirtualRange(0, 40, 0, 400)).toEqual({
      start: 0,
      end: 0,
      offsetTop: 0,
      totalHeight: 0,
      visibleCount: 0,
    });
  });

  it("windows the middle of a long list", () => {
    const r = computeVirtualRange(200, 40, 800, 400, 2);
    // scrollTop 800 → index 20; overscan 2 → start 18
    expect(r.start).toBe(18);
    expect(r.offsetTop).toBe(18 * 40);
    expect(r.totalHeight).toBe(200 * 40);
    expect(r.end).toBeGreaterThan(r.start);
    expect(r.end).toBeLessThanOrEqual(200);
    expect(r.visibleCount).toBe(r.end - r.start);
  });

  it("clamps start to 0 at top", () => {
    const r = computeVirtualRange(50, 40, 0, 200, 5);
    expect(r.start).toBe(0);
    expect(r.offsetTop).toBe(0);
  });

  it("clamps end to count at bottom", () => {
    const r = computeVirtualRange(10, 40, 10000, 400, 2);
    expect(r.end).toBe(10);
    expect(r.start).toBeLessThanOrEqual(10);
  });

  it("exports a sensible threshold", () => {
    expect(VIRTUAL_THRESHOLD).toBeGreaterThanOrEqual(20);
  });
});
