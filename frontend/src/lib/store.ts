"use client";

import { create } from "zustand";
import { api } from "./api";
import type { Listener } from "@/types/listener";
import type { DashboardStats } from "@/types/agent";

export interface OnlineUser {
  username: string;
  connected_at: string;
}

interface AppState {
  listeners: Listener[];
  stats: DashboardStats | null;
  statsError?: string;
  loading: boolean;

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

  fetchListeners: () => Promise<void>;
  fetchStats: () => Promise<void>;
}

export const useAppStore = create<AppState>((set, get) => ({
  listeners: [],
  stats: null,
  loading: false,
  onlineUsers: [],
  currentUsername: "",

  sidebarCollapsed: false,
  isMobile: false,
  mobileMenuOpen: false,

  toggleSidebar: () => {
    const { isMobile, mobileMenuOpen, sidebarCollapsed } = get();
    if (isMobile) set({ mobileMenuOpen: !mobileMenuOpen });
    else set({ sidebarCollapsed: !sidebarCollapsed });
  },

  setMobileMenuOpen: (open: boolean) => set({ mobileMenuOpen: open }),
  setIsMobile: (mobile: boolean) =>
    set({ isMobile: mobile, mobileMenuOpen: mobile ? false : get().mobileMenuOpen }),

  getSidebarWidth: () => {
    const { isMobile, mobileMenuOpen, sidebarCollapsed } = get();
    if (isMobile && !mobileMenuOpen) return 0;
    return sidebarCollapsed ? 64 : 224;
  },

  setOnlineUsers: (users) => set({ onlineUsers: users }),
  setCurrentUsername: (name) => set({ currentUsername: name }),

  fetchListeners: async () => {
    try {
      const data = await api.get<{ listeners?: Listener[]; Listeners?: Listener[] }>("/listeners");
      const listeners = data.listeners || data.Listeners || [];
      set({ listeners });
    } catch {
      set({ listeners: [] });
    }
  },

  fetchStats: async () => {
    try {
      const stats = await api.get<DashboardStats>("/dashboard");
      set({ stats, statsError: undefined });
    } catch (e) {
      console.error("[store] fetchStats failed", e);
      set({ statsError: e instanceof Error ? e.message : "Failed to load stats" });
    }
  },
}));
