"use client";

import type { StateCreator } from "zustand";
import type { AppStore, OnlineUser } from "./types";

export interface AuthSlice {
  onlineUsers: OnlineUser[];
  setOnlineUsers: (users: OnlineUser[]) => void;
  currentUsername: string;
  setCurrentUsername: (name: string) => void;
  currentUserRole: string;
  setCurrentUserRole: (role: string) => void;
  currentPermissions: string[] | null;
  setCurrentPermissions: (perms: string[]) => void;
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