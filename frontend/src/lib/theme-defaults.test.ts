import { describe, expect, it } from "vitest";
import { DEFAULT_THEME, resolveStoredTheme } from "./theme-defaults";

describe("resolveStoredTheme", () => {
  it("defaults missing storage to light", () => {
    expect(DEFAULT_THEME).toBe("light");
    expect(resolveStoredTheme(null)).toBe("light");
    expect(resolveStoredTheme(undefined)).toBe("light");
    expect(resolveStoredTheme("")).toBe("light");
    expect(resolveStoredTheme("nope")).toBe("light");
  });

  it("preserves an explicit user choice", () => {
    expect(resolveStoredTheme("light")).toBe("light");
    expect(resolveStoredTheme("system")).toBe("system");
    expect(resolveStoredTheme("dark")).toBe("dark");
  });
});
