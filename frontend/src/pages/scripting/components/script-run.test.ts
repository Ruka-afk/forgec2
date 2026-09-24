import { describe, expect, it } from "vitest";
import {
  extractRunHistory,
  extractSavedScripts,
  normalizeScriptOutput,
  scriptRunError,
} from "./script-run";

describe("extractSavedScripts", () => {
  it("accepts the unwrapped bare array from GET /api/scripts", () => {
    const scripts = [{ id: "1", name: "recon", code: "1" }];
    expect(extractSavedScripts(scripts)).toEqual(scripts);
  });

  it("falls back to keyed shapes instead of returning nothing", () => {
    expect(extractSavedScripts({ scripts: [{ id: "1" }] })).toHaveLength(1);
    expect(extractSavedScripts({ data: [{ id: "2" }] })).toHaveLength(1);
  });

  it("returns an empty list for unusable payloads", () => {
    expect(extractSavedScripts(null)).toEqual([]);
    expect(extractSavedScripts("nope")).toEqual([]);
    expect(extractSavedScripts({ data: { nope: true } })).toEqual([]);
  });
});

describe("extractRunHistory", () => {
  it("reads the history key from the non-unwrapped envelope", () => {
    expect(extractRunHistory({ success: true, history: [{ id: 4 }] })).toHaveLength(1);
  });

  it("returns an empty list when history is absent", () => {
    expect(extractRunHistory({ success: true })).toEqual([]);
    expect(extractRunHistory(undefined)).toEqual([]);
  });
});

describe("normalizeScriptOutput", () => {
  it("reads output out of the nested result object", () => {
    const payload = { success: true, result: { success: true, output: "hello", error: "" } };
    expect(normalizeScriptOutput(payload, "fallback")).toBe("hello");
  });

  it("never returns an object, which would crash React child rendering", () => {
    const payload = { success: true, result: { success: true, output: { a: 1 } } };
    const out = normalizeScriptOutput(payload, "fallback");
    expect(typeof out).toBe("string");
    expect(out).toBe('{\n  "a": 1\n}');
  });

  it("prefers the error text over output", () => {
    const payload = { success: true, result: { success: false, output: "", error: "SyntaxError" } };
    expect(normalizeScriptOutput(payload, "fallback")).toBe("SyntaxError");
    expect(scriptRunError(payload)).toBe("SyntaxError");
  });

  it("falls back when there is no output at all", () => {
    expect(normalizeScriptOutput({ success: true, result: { success: true } }, "fallback")).toBe("fallback");
    expect(normalizeScriptOutput({}, "fallback")).toBe("fallback");
    expect(normalizeScriptOutput(null, "fallback")).toBe("fallback");
  });

  it("accepts a flat result shape and a raw string", () => {
    expect(normalizeScriptOutput({ output: "flat" }, "fallback")).toBe("flat");
    expect(normalizeScriptOutput({ data: { output: "wrapped" } }, "fallback")).toBe("wrapped");
    expect(normalizeScriptOutput("raw", "fallback")).toBe("raw");
  });
});
