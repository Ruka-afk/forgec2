import { describe, expect, it } from "vitest";
import { isFlushPath, showBreadcrumbBar } from "./layout";

describe("isFlushPath", () => {
  it("marks workspace routes as flush", () => {
    expect(isFlushPath("/chat")).toBe(true);
    expect(isFlushPath("/ai")).toBe(true);
    expect(isFlushPath("/agents/abc/shell")).toBe(true);
    expect(isFlushPath("/agents/abc/files")).toBe(true);
    expect(isFlushPath("/agents/abc/screen")).toBe(true);
    expect(isFlushPath("/agents/abc/remote-desktop")).toBe(true);
  });

  it("keeps document pages in the padded column", () => {
    expect(isFlushPath("/agents")).toBe(false);
    expect(isFlushPath("/agents/abc")).toBe(false);
    expect(isFlushPath("/agents/abc/config")).toBe(false);
    expect(isFlushPath("/dashboard")).toBe(false);
    expect(isFlushPath("/listeners")).toBe(false);
  });
});

describe("showBreadcrumbBar", () => {
  it("hides on dashboard, focus mode, and flush workspaces", () => {
    expect(showBreadcrumbBar("/dashboard", false)).toBe(false);
    expect(showBreadcrumbBar("/agents", true)).toBe(false);
    expect(showBreadcrumbBar("/chat", false)).toBe(false);
    expect(showBreadcrumbBar("/agents", false)).toBe(true);
    expect(showBreadcrumbBar("/listeners/1", false)).toBe(true);
  });
});
