"use client";

// Central registry of UI polling cadences (ms). Named by concern so call sites
// read intent instead of magic numbers; tuned together as one table.
export const POLL = {
  /** Global dashboard stats (sidebar badges, dashboards). */
  stats: 30_000,
  /** Agents/beacons list auto-refresh. */
  agents: 30_000,
  /** Task center grid poll. */
  tasks: 30_000,
  /** Agent-detail task drift correction between WS merges. */
  taskCorrection: 30_000,
  /** Topology graph refresh. */
  topology: 10_000,
  /** Privesc / misc listings. */
  listing: 10_000,
  /** Active-scan fast poll while a scan is running. */
  scanActive: 3_000,
  /** Fallback task poll while the WebSocket is down. */
  wsDownPoll: 8_000,
  /** Relative-time "now" ticker. */
  clockTick: 15_000,
} as const;