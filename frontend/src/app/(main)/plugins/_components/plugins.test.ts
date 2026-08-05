import { describe, it, expect } from "vitest";
import { normalizePluginList, pluginId } from "./types";

describe("normalizePluginList", () => {
  it("accepts bare arrays", () => {
    expect(normalizePluginList([{ id: "1" }])).toHaveLength(1);
  });
  it("unwraps plugins key", () => {
    expect(normalizePluginList({ plugins: [{ id: "a" }] })[0]).toMatchObject({ id: "a" });
  });
  it("empty for junk", () => {
    expect(normalizePluginList(null)).toEqual([]);
  });
});

describe("pluginId", () => {
  it("prefers ID then id", () => {
    expect(pluginId({ ID: "X" })).toBe("X");
    expect(pluginId({ id: "y" })).toBe("y");
  });
});
