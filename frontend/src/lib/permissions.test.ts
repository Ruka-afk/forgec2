import { describe, it, expect } from "vitest";
import { can, canAny, canAll } from "./permissions";

const PERMS = ["agents.read", "agents.write", "users.read", "settings.read"] as const;

describe("can", () => {
  it("returns true when the permission is held", () => {
    expect(can(PERMS, "agents.read")).toBe(true);
  });

  it("returns false when the permission is missing", () => {
    expect(can(PERMS, "users.delete")).toBe(false);
  });

  it("returns false for null/undefined permission lists", () => {
    expect(can(null, "agents.read")).toBe(false);
    expect(can(undefined, "agents.read")).toBe(false);
  });
});

describe("canAny", () => {
  it("returns true when any permission is held", () => {
    expect(canAny(PERMS, ["users.write", "users.read"] as const)).toBe(true);
  });

  it("returns false when none are held", () => {
    expect(canAny(PERMS, ["users.delete", "plugins.read"] as const)).toBe(false);
  });

  it("returns false for an empty required list", () => {
    expect(canAny(PERMS, [] as const)).toBe(false);
  });
});

describe("canAll", () => {
  it("returns true when every permission is held", () => {
    expect(canAll(PERMS, ["agents.read", "settings.read"] as const)).toBe(true);
  });

  it("returns false when any permission is missing", () => {
    expect(canAll(PERMS, ["agents.read", "users.delete"] as const)).toBe(false);
  });
});