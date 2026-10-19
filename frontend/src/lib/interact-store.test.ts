import { describe, expect, it } from "vitest";
import { isAgentSessionPath, nextRevealKind, shouldCloseOnNavigate } from "./interact-store";

describe("shouldCloseOnNavigate", () => {
  it("keeps a pinned session when the route changes", () => {
    expect(shouldCloseOnNavigate(true, "/beacons", "/loot")).toBe(false);
  });
  it("closes an unpinned session on navigation", () => {
    expect(shouldCloseOnNavigate(false, "/beacons", "/loot")).toBe(true);
  });
  it("does not close on the same path", () => {
    expect(shouldCloseOnNavigate(false, "/beacons", "/beacons")).toBe(false);
  });
  it("keeps an unpinned dock when entering that session's dest", () => {
    expect(shouldCloseOnNavigate(false, "/beacons", "/beacons/abc", "abc")).toBe(false);
    expect(shouldCloseOnNavigate(false, "/beacons/abc", "/beacons/abc/files", "abc")).toBe(false);
  });
  it("closes when leaving that session", () => {
    expect(shouldCloseOnNavigate(false, "/beacons/abc", "/beacons", "abc")).toBe(true);
    expect(shouldCloseOnNavigate(false, "/beacons/abc", "/beacons/other", "abc")).toBe(true);
  });
});

describe("nextRevealKind", () => {
  it("reveals only when the dock is already that session", () => {
    expect(nextRevealKind("abc", "abc")).toBe("reveal");
    expect(nextRevealKind(null, "abc")).toBe("offer");
    expect(nextRevealKind("other", "abc")).toBe("offer");
  });
});

describe("isAgentSessionPath", () => {
  it("matches the session and its dests only", () => {
    expect(isAgentSessionPath("/beacons/abc", "abc")).toBe(true);
    expect(isAgentSessionPath("/beacons/abc/shell", "abc")).toBe(true);
    expect(isAgentSessionPath("/beacons/abcd", "abc")).toBe(false);
    expect(isAgentSessionPath("/beacons", "abc")).toBe(false);
  });
});
