type ListenerHealthStatus = "healthy" | "unstable" | "burned" | "unknown";

export interface ListenerHealth {
  target: string;
  scheme?: string;
  host?: string;
  port?: number;
  status?: string;
  consecutive_fails?: number;
  last_probe?: string;
  fail_reasons?: string[];
}

/** Probe target id is the listener numeric id as a decimal string. */
export function listenerHealthKey(listenerId: string | number | undefined | null): string {
  if (listenerId == null || listenerId === "") return "";
  return String(listenerId);
}

/** Index probe rows by listener id. Redirector targets (`redirector:N`) are ignored. */
export function indexListenerHealth(items: ListenerHealth[]): Record<string, ListenerHealth> {
  const map: Record<string, ListenerHealth> = {};
  for (const item of items) {
    const target = item?.target;
    if (!target || target.startsWith("redirector:")) continue;
    map[target] = item;
  }
  return map;
}

export function healthForListener(
  healthByTarget: Record<string, ListenerHealth>,
  listenerId: string | number | undefined | null,
): ListenerHealth | undefined {
  const key = listenerHealthKey(listenerId);
  if (!key) return undefined;
  return healthByTarget[key];
}

export function isProblemHealth(health?: ListenerHealth): boolean {
  if (!health) return false;
  return health.status === "unstable" || health.status === "burned";
}

export function healthIndicatorStatus(status?: string): ListenerHealthStatus {
  if (status === "healthy" || status === "unstable" || status === "burned") return status;
  return "unknown";
}

export function translateHealthStatus(
  t: (key: string, params?: Record<string, string | number>) => string,
  status?: string,
  monitored = true,
): string {
  if (!monitored) return t("listeners.health_unmonitored");
  switch (status) {
    case "healthy":
      return t("listeners.health_healthy");
    case "unstable":
      return t("listeners.health_unstable");
    case "burned":
      return t("listeners.health_burned");
    default:
      return t("listeners.health_unknown");
  }
}

/** One polled observation of a listener's circuit-breaker status. */
export interface HealthSample {
  /** Epoch ms when this sample was observed on the client. */
  t: number;
  status: string;
}

/** Ring-buffer cap so long-lived list pages stay bounded. */
export const HEALTH_HISTORY_MAX = 48;

/**
 * Merge one poll tick into the per-listener sample history.
 *
 * - Always records a sample when status changes (so transitions are visible).
 * - Otherwise only records when `minIntervalMs` has elapsed (avoids a bar per
 *   identical poll when the list refresh cadence is faster than probes).
 * - Returns `prev` unchanged when nothing was appended (stable React refs).
 */
export function appendHealthSamples(
  prev: Record<string, HealthSample[]>,
  health: Record<string, ListenerHealth>,
  opts: { now?: number; max?: number; minIntervalMs?: number } = {},
): Record<string, HealthSample[]> {
  const now = opts.now ?? Date.now();
  const max = Math.max(2, opts.max ?? HEALTH_HISTORY_MAX);
  const minIntervalMs = opts.minIntervalMs ?? 14_000;
  let changed = false;
  const next: Record<string, HealthSample[]> = { ...prev };

  for (const key of Object.keys(health)) {
    const status = healthIndicatorStatus(health[key]?.status);
    const samples = next[key] ?? [];
    const last = samples[samples.length - 1];
    if (last && last.status === status && now - last.t < minIntervalMs) continue;
    const appended = [...samples, { t: now, status }];
    next[key] = appended.length > max ? appended.slice(appended.length - max) : appended;
    changed = true;
  }

  return changed ? next : prev;
}
