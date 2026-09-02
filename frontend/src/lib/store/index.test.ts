import { describe, expect, it } from "vitest";
import { selectSidebarWidth, useAppStore } from "./index";

function state(overrides: Partial<ReturnType<typeof useAppStore.getState>>) {
  return { ...useAppStore.getState(), ...overrides };
}

describe("selectSidebarWidth", () => {
  it("uses the configured desktop sidebar width", () => {
    expect(selectSidebarWidth(state({ isMobile: false, sidebarCollapsed: false }))).toBe(224);
    expect(selectSidebarWidth(state({ isMobile: false, sidebarCollapsed: true }))).toBe(64);
  });

  it("does not offset layout for the mobile overlay", () => {
    expect(selectSidebarWidth(state({ isMobile: true, mobileMenuOpen: false }))).toBe(0);
    expect(selectSidebarWidth(state({ isMobile: true, mobileMenuOpen: true }))).toBe(0);
  });
});
