"use client";

import type { StateCreator } from "zustand";
import type { AppStore, Density } from "./types";

interface UiSlice {
  sidebarCollapsed: boolean;
  isMobile: boolean;
  mobileMenuOpen: boolean;
  toggleSidebar: () => void;
  setMobileMenuOpen: (open: boolean) => void;
  setIsMobile: (mobile: boolean) => void;
  commandPaletteOpen: boolean;
  setCommandPaletteOpen: (open: boolean | ((v: boolean) => boolean)) => void;
  density: Density;
  setDensity: (d: Density) => void;
  focusMode: boolean;
  toggleFocusMode: () => void;
  setFocusMode: (b: boolean) => void;
}

// Legacy localStorage keys — read once to seed values for users who set prefs
// before the persist middleware existed. New writes go through the persist
// blob ("forgec2_ui_prefs_v1").
const SIDEBAR_LEGACY_KEY = "forgec2_sidebar_collapsed";
const DENSITY_LEGACY_KEY = "forgec2_density";
const FOCUS_LEGACY_KEY = "forgec2_focus_mode";

function legacyBool(key: string): boolean | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(key);
  if (raw === null) return null;
  return raw === "true";
}

function legacyDensity(): Density | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(DENSITY_LEGACY_KEY);
  if (raw === "compact" || raw === "comfortable") return raw;
  return null;
}

function applyDensity(d: Density) {
  if (typeof document === "undefined") return;
  document.documentElement.setAttribute("data-density", d);
}
function applyFocus(on: boolean) {
  if (typeof document === "undefined") return;
  document.documentElement.setAttribute("data-focus", on ? "on" : "off");
}

export const createUiSlice: StateCreator<AppStore, [], [], UiSlice> = (set, get) => ({
  sidebarCollapsed: legacyBool(SIDEBAR_LEGACY_KEY) ?? false,
  isMobile: false,
  mobileMenuOpen: false,
  commandPaletteOpen: false,
  density: legacyDensity() ?? "comfortable",
  focusMode: legacyBool(FOCUS_LEGACY_KEY) ?? false,

  toggleSidebar: () => {
    const { isMobile, mobileMenuOpen, sidebarCollapsed } = get();
    if (isMobile) set({ mobileMenuOpen: !mobileMenuOpen });
    else set({ sidebarCollapsed: !sidebarCollapsed });
  },

  setMobileMenuOpen: (open: boolean) => set({ mobileMenuOpen: open }),
  setIsMobile: (mobile: boolean) =>
    set({ isMobile: mobile, mobileMenuOpen: mobile ? false : get().mobileMenuOpen }),

  setCommandPaletteOpen: (open: boolean | ((v: boolean) => boolean)) =>
    set((state) => ({
      commandPaletteOpen: typeof open === "function" ? (open as (v: boolean) => boolean)(state.commandPaletteOpen) : open,
    })),

  setDensity: (d) => {
    applyDensity(d);
    set({ density: d });
  },
  toggleFocusMode: () => {
    const next = !get().focusMode;
    applyFocus(next);
    set({ focusMode: next });
  },
  setFocusMode: (b) => {
    applyFocus(b);
    set({ focusMode: b });
  },
});