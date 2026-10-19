import { describe, expect, it } from "vitest";
import { isFlushPath, showBreadcrumbBar } from "./layout";

describe("isFlushPath", () => {
  it("marks workspace routes as flush", () => {
    expect(isFlushPath("/chat")).toBe(true);
    expect(isFlushPath("/ai")).toBe(true);
    expect(isFlushPath("/beacons/abc/shell")).toBe(true);
    expect(isFlushPath("/beacons/abc/files")).toBe(true);
    expect(isFlushPath("/beacons/abc/screen")).toBe(true);
    expect(isFlushPath("/beacons/abc/remote-desktop")).toBe(true);
  });

  it("keeps document pages in the padded column", () => {
    expect(isFlushPath("/beacons")).toBe(false);
    expect(isFlushPath("/beacons/abc")).toBe(false);
    expect(isFlushPath("/beacons/abc/config")).toBe(false);
    expect(isFlushPath("/dashboard")).toBe(false);
    expect(isFlushPath("/listeners")).toBe(false);
  });
});

describe("showBreadcrumbBar", () => {
  it("hides on dashboard, focus mode, and flush workspaces", () => {
    expect(showBreadcrumbBar("/dashboard", false)).toBe(false);
    expect(showBreadcrumbBar("/beacons", true)).toBe(false);
    expect(showBreadcrumbBar("/chat", false)).toBe(false);
    expect(showBreadcrumbBar("/beacons", false)).toBe(true);
    expect(showBreadcrumbBar("/listeners/1", false)).toBe(true);
  });
});
