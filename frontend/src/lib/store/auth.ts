"use client";

import type { StateCreator } from "zustand";
import type { AppStore, OnlineUser } from "./types";
import type { PermissionKey } from "@/lib/permission-keys";

export interface AuthSlice {
  onlineUsers: OnlineUser[];
  setOnlineUsers: (users: OnlineUser[]) => void;
  currentUsername: string;
  setCurrentUsername: (name: string) => void;
  currentUserRole: string;
  setCurrentUserRole: (role: string) => void;
  currentPermissions: PermissionKey[] | null;
  setCurrentPermissions: (perms: PermissionKey[]) => void;
}

export const createAuthSlice: StateCreator<AppStore, [], [], AuthSlice> = (set) => ({
  onlineUsers: [],
  currentUsername: "",
  currentUserRole: "",
  currentPermissions: null,

  setOnlineUsers: (users) => set({ onlineUsers: users }),
  setCurrentUsername: (name) => set({ currentUsername: name }),
  setCurrentUserRole: (role) => set({ currentUserRole: role }),
  setCurrentPermissions: (perms) => set({ currentPermissions: perms }),
});