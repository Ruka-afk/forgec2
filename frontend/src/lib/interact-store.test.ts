import { describe, expect, it } from "vitest";
import { isAgentSessionPath, nextRevealKind, shouldCloseOnNavigate } from "./interact-store";

describe("shouldCloseOnNavigate", () => {
  it("keeps a pinned session when the route changes", () => {
    expect(shouldCloseOnNavigate(true, "/agents", "/loot")).toBe(false);
  });
  it("closes an unpinned session on navigation", () => {
    expect(shouldCloseOnNavigate(false, "/agents", "/loot")).toBe(true);
  });
  it("does not close on the same path", () => {
    expect(shouldCloseOnNavigate(false, "/agents", "/agents")).toBe(false);
  });
  it("keeps an unpinned dock when entering that session's dest", () => {
    expect(shouldCloseOnNavigate(false, "/agents", "/agents/abc", "abc")).toBe(false);
    expect(shouldCloseOnNavigate(false, "/agents/abc", "/agents/abc/files", "abc")).toBe(false);
  });
  it("closes when leaving that session", () => {
    expect(shouldCloseOnNavigate(false, "/agents/abc", "/agents", "abc")).toBe(true);
    expect(shouldCloseOnNavigate(false, "/agents/abc", "/agents/other", "abc")).toBe(true);
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
    expect(isAgentSessionPath("/agents/abc", "abc")).toBe(true);
    expect(isAgentSessionPath("/agents/abc/shell", "abc")).toBe(true);
    expect(isAgentSessionPath("/agents/abcd", "abc")).toBe(false);
    expect(isAgentSessionPath("/agents", "abc")).toBe(false);
  });
});
