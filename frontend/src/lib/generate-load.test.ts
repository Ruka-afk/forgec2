import { describe, it, expect } from "vitest";
import { normalizeListeners, extractProfilePresets, canStartGenerate } from "./generate-load";

describe("normalizeListeners", () => {
  it("accepts bare arrays", () => {
    expect(normalizeListeners([{ id: 1 }])).toEqual([{ id: 1 }]);
  });
  it("unwraps listeners key", () => {
    expect(normalizeListeners({ listeners: [{ id: 2 }] })).toEqual([{ id: 2 }]);
  });
  it("unwraps data key", () => {
    expect(normalizeListeners({ data: [{ id: 3 }] })).toEqual([{ id: 3 }]);
  });
  it("empty for junk", () => {
    expect(normalizeListeners(null)).toEqual([]);
    expect(normalizeListeners({})).toEqual([]);
  });
});

describe("extractProfilePresets", () => {
  const fb = [{ name: "default", description: "d", user_agent: "", sleep: 0, jitter: 0 }];
  it("uses success+data.profiles", () => {
    const got = extractProfilePresets(
      { success: true, data: { profiles: [{ name: "x", description: "", user_agent: "", sleep: 1, jitter: 2 }] } },
      fb,
    );
    expect(got[0].name).toBe("x");
  });
  it("falls back when empty", () => {
    expect(extractProfilePresets({ success: true, data: { profiles: [] } }, fb)).toBe(fb);
  });
});

describe("canStartGenerate", () => {
  it("requires non-empty listener", () => {
    expect(canStartGenerate("")).toBe(false);
    expect(canStartGenerate("  ")).toBe(false);
    expect(canStartGenerate(null)).toBe(false);
    expect(canStartGenerate("12")).toBe(true);
  });
});
