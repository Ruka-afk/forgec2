import { describe, it, expect } from "vitest";
import { normalizeAgentList } from "./agents";

describe("normalizeAgentList", () => {
  it("returns bare arrays as-is", () => {
    const list = [{ id: "a1" }, { id: "a2" }];
    expect(normalizeAgentList(list)).toEqual(list);
  });

  it("unwraps agents key", () => {
    const agents = [{ id: "x" }];
    expect(normalizeAgentList({ agents })).toEqual(agents);
  });

  it("unwraps Agents key", () => {
    const Agents = [{ id: "y" }];
    expect(normalizeAgentList({ Agents })).toEqual(Agents);
  });

  it("unwraps data key", () => {
    const data = [{ id: "z" }];
    expect(normalizeAgentList({ data })).toEqual(data);
  });

  it("returns empty array for null/undefined/non-object", () => {
    expect(normalizeAgentList(null)).toEqual([]);
    expect(normalizeAgentList(undefined)).toEqual([]);
    expect(normalizeAgentList("nope")).toEqual([]);
    expect(normalizeAgentList(42)).toEqual([]);
  });

  it("returns empty array for object without list keys", () => {
    expect(normalizeAgentList({ total: 0 })).toEqual([]);
  });
});
