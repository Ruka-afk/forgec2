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
