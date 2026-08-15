import { describe, expect, it } from "vitest";
import { clampMenuPoint } from "./agent-menu-actions";

describe("clampMenuPoint", () => {
  it("keeps a point inside the viewport", () => {
    const pos = clampMenuPoint(40, 50, 200, 200);
    expect(pos.x).toBeGreaterThanOrEqual(8);
    expect(pos.y).toBeGreaterThanOrEqual(8);
  });
});
