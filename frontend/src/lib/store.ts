"use client";

import { create } from "zustand";
import { api } from "./api";
import { paths } from "./api-paths";
import type { DashboardStats } from "@/types/agent";

export interface OnlineUser {
  username: string;
  connected_at: string;
}

interface AppState {
  stats: DashboardStats | null;
  statsError?: string;

  // Sidebar layout state
  sidebarCollapsed: boolean;
  isMobile: boolean;
  mobileMenuOpen: boolean;
  toggleSidebar: () => void;
  setMobileMenuOpen: (open: boolean) => void;
  setIsMobile: (mobile: boolean) => void;
  getSidebarWidth: () => number;

  // Online users
  onlineUsers: OnlineUser[];
  setOnlineUsers: (users: OnlineUser[]) => void;
  currentUsername: string;
  setCurrentUsername: (name: string) => void;
  currentUserRole: string;
  setCurrentUserRole: (role: string) => void;

  fetchStats: () => Promise<void>;
}

export const useAppStore = create<AppState>((set, get) => ({
  stats: null,
  onlineUsers: [],
  currentUsername: "",
  currentUserRole: "",

  sidebarCollapsed: typeof window !== "undefined"
    ? localStorage.getItem("forgec2_sidebar_collapsed") === "true"
    : false,
  isMobile: false,
  mobileMenuOpen: false,

  toggleSidebar: () => {
    const { isMobile, mobileMenuOpen, sidebarCollapsed } = get();
    if (isMobile) set({ mobileMenuOpen: !mobileMenuOpen });
    else {
      const next = !sidebarCollapsed;
      localStorage.setItem("forgec2_sidebar_collapsed", String(next));
      set({ sidebarCollapsed: next });
    }
  },

  setMobileMenuOpen: (open: boolean) => set({ mobileMenuOpen: open }),
  setIsMobile: (mobile: boolean) =>
    set({ isMobile: mobile, mobileMenuOpen: mobile ? false : get().mobileMenuOpen }),

  getSidebarWidth: () => {
    const state = get();
    if (state.isMobile && !state.mobileMenuOpen) return 0;
    return state.sidebarCollapsed ? 64 : 192;
  },

  setOnlineUsers: (users) => set({ onlineUsers: users }),
  setCurrentUsername: (name) => set({ currentUsername: name }),
  setCurrentUserRole: (role) => set({ currentUserRole: role }),

  fetchStats: async () => {
    try {
      const stats = await api.get<DashboardStats>(paths.dashboard.v1);
      set({ stats, statsError: undefined });
    } catch (e) {
      if (process.env.NODE_ENV === "development") console.error("[store] fetchStats failed", e);
      set({ statsError: e instanceof Error ? e.message : "Failed to load stats" });
    }
  },
}));
