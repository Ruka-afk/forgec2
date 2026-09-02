"use client";

import type { DashboardStats } from "@/types/agent";
import type { PermissionKey } from "@/lib/permission-keys";

export interface OnlineUser {
  username: string;
  connected_at: string;
}

export type Density = "comfortable" | "compact";

export interface AppStore {
  // Stats (stats slice)
  stats: DashboardStats | null;
  statsError?: string;
  fetchStats: () => Promise<void>;

  // Auth / identity (auth slice)
  onlineUsers: OnlineUser[];
  setOnlineUsers: (users: OnlineUser[]) => void;
  currentUsername: string;
  setCurrentUsername: (name: string) => void;
  currentUserRole: string;
  setCurrentUserRole: (role: string) => void;
  /** null = not yet loaded from /api/me; UI gates fail open until known. */
  currentPermissions: PermissionKey[] | null;
  setCurrentPermissions: (perms: PermissionKey[]) => void;

  // UI preferences (ui slice)
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