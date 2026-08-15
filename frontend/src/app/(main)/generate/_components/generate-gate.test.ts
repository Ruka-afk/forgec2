import { describe, expect, it } from "vitest";
import { canGenerateFromListener } from "./generate-gate";

describe("canGenerateFromListener", () => {
  it("disables generate until a listener id is present", () => {
    expect(canGenerateFromListener("")).toBe(false);
    expect(canGenerateFromListener("   ")).toBe(false);
    expect(canGenerateFromListener(null)).toBe(false);
    expect(canGenerateFromListener(undefined)).toBe(false);
    expect(canGenerateFromListener("12")).toBe(true);
  });
});
