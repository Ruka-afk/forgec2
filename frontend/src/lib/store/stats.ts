"use client";

import type { StateCreator } from "zustand";
import { api } from "../api";
import { paths } from "../api-paths";
import { logger } from "../logger";
import type { DashboardStats } from "@/types/agent";
import type { AppStore } from "./types";

export interface StatsSlice {
  stats: DashboardStats | null;
  statsError?: string;
  fetchStats: () => Promise<void>;
}

let statsInFlight: Promise<void> | null = null;

export const createStatsSlice: StateCreator<AppStore, [], [], StatsSlice> = (set) => ({
  stats: null,

  fetchStats: async () => {
    if (statsInFlight) return statsInFlight;
    statsInFlight = (async () => {
      try {
        const stats = await api.get<DashboardStats>(paths.dashboard.v1);
        set({ stats, statsError: undefined });
      } catch (e) {
        if (process.env.NODE_ENV === "development") logger.error("fetchStats failed", e);
        set({ statsError: e instanceof Error ? e.message : "" });
      } finally {
        statsInFlight = null;
      }
    })();
    return statsInFlight;
  },
});