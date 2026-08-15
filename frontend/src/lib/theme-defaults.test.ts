import { describe, expect, it } from "vitest";
import { DEFAULT_THEME, resolveStoredTheme } from "./theme-defaults";

describe("resolveStoredTheme", () => {
  it("defaults missing storage to dark", () => {
    expect(DEFAULT_THEME).toBe("dark");
    expect(resolveStoredTheme(null)).toBe("dark");
    expect(resolveStoredTheme(undefined)).toBe("dark");
    expect(resolveStoredTheme("")).toBe("dark");
    expect(resolveStoredTheme("nope")).toBe("dark");
  });

  it("preserves an explicit user choice", () => {
    expect(resolveStoredTheme("light")).toBe("light");
    expect(resolveStoredTheme("system")).toBe("system");
    expect(resolveStoredTheme("dark")).toBe("dark");
  });
});
