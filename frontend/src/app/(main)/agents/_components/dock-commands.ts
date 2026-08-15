import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

export const DOCK_COMMANDS = ["ps", "screenshot", "sleep"] as const;
export type DockCommandKind = (typeof DOCK_COMMANDS)[number];

export function parseSleepArgs(raw: string): { interval: number; jitter: number } | null {
  const parts = raw.trim().split(/[\s,]+/).filter(Boolean);
  if (parts.length === 0) return null;
  const interval = Number(parts[0]);
  const jitter = parts.length > 1 ? Number(parts[1]) : 0;
  if (!Number.isFinite(interval) || interval < 1) return null;
  if (!Number.isFinite(jitter) || jitter < 0 || jitter > 100) return null;
  return { interval: Math.round(interval), jitter: Math.round(jitter) };
}

export function dockCommandLabel(kind: DockCommandKind): string {
  if (kind === "ps") return "ps";
  if (kind === "screenshot") return "screenshot";
  return "set_sleep";
}

export function parseSocksPort(raw: string): number | null {
  const n = Number(String(raw).trim());
  if (!Number.isInteger(n) || n < 1 || n > 65535) return null;
  return n;
}

export async function socksRelayStatus(agentId: string): Promise<{ active: boolean; port: number | null }> {
  const data = await api.get<{ active?: boolean; port?: number }>(paths.agents.socksRelayStatus(agentId));
  const port = Number(data.port);
  return {
    active: data.active === true,
    port: Number.isFinite(port) && port > 0 ? port : null,
  };
}

export async function startDockSocks(agentId: string, port: number): Promise<{ port: number; message: string }> {
  const data = await api.post<{ port?: number; message?: string }>(paths.agents.socksRelayStart(agentId), {
    port: String(port),
  });
  const actual = Number(data.port);
  return {
    port: Number.isFinite(actual) && actual > 0 ? actual : port,
    message: String(data.message || ""),
  };
}

export async function stopDockSocks(agentId: string): Promise<void> {
  await api.post(paths.agents.socksRelayStop(agentId), { agent_id: agentId });
}

export async function queueDockCommand(
  agentId: string,
  kind: DockCommandKind,
  sleep?: { interval: number; jitter: number },
): Promise<{ task_id: number; type: string; command: string }> {
  let data: { task_id?: number };
  let command = "";
  if (kind === "ps") {
    data = await api.post(paths.agents.cmd(agentId, "ps"));
    command = "ps";
  } else if (kind === "screenshot") {
    data = await api.post(paths.agents.screenshotTask(agentId));
    command = "screenshot";
  } else {
    const interval = sleep?.interval ?? 60;
    const jitter = sleep?.jitter ?? 0;
    command = `${interval},${jitter}`;
    data = await api.postJson(paths.agents.setSleep(agentId), { interval, jitter });
  }
  const taskId = Number(data.task_id);
  if (!Number.isFinite(taskId) || taskId <= 0) {
    throw new Error("no task_id");
  }
  return { task_id: taskId, type: dockCommandLabel(kind), command };
}
