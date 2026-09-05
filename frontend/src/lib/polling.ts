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
  /** Shell page agent status (online/offline overlay). */
  shellStatus: 5_000,
  /** Relative-time "now" ticker. */
  clockTick: 15_000,
  /** Circuit-breaker breaker state page. */
  circuitBreaker: 15_000,

  /** DNS record/listener status page (nearly static; refresh on toggle). */
  dns: 30_000,
  /** Password-spray agents listing. */
  spray: 30_000,
  /** Dashboard ops aggregate fetch (outside the WS stats stream). */
  opsHome: 15_000,
  /** Listener list page. */
  listeners: 15_000,
  /** Report run data page. */
  report: 30_000,
  /** Pivoting relay/forward status page (nearly static; refresh on action). */
  pivoting: 30_000,
  /** Traffic page auto-refresh toggle. */
  traffic: 5_000,
  /** Dashboard active-missions board. */
  missions: 30_000,
  /** Timeline events stream while the WS is connected. */
  events: 30_000,
  /** Timeline events stream fallback while the WS is down. */
  eventsFallback: 10_000,
  /** Error-toast throttle window for polling fetchers (default cadence). */
  toastThrottle: 10_000,
  /** Error-toast throttle for slow-polling (30s) surfaces. */
  toastThrottleLong: 30_000,
  /** Error-toast throttle for fast-polling (5s) surfaces. */
  toastThrottleShort: 5_000,
  /** Error-toast throttle for alert/notification surfaces. */
  toastThrottleAlerts: 15_000,
  /** Cloud credential-steal progress poll while an operation is in flight. */
  stealPoll: 3_000,
} as const;