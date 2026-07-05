import { API_BASE } from "./constants";

export function decodeShellResult(data: { result?: string; encoding?: string }): string {
  let out = data?.result ? String(data.result) : "";
  if (!out.trim()) return out;
  const isB64 =
    data.encoding === "base64" ||
    (out.length > 40 && /^[A-Za-z0-9+/=\s]+$/.test(out.trim()));
  if (!isB64) return out;
  try {
    const binary = atob(out.replace(/\s/g, ""));
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return new TextDecoder("utf-8").decode(bytes);
  } catch {
    return out;
  }
}

export function getShellTiming(interval: number, jitter: number) {
  if (interval === 0) {
    return { interactive: true, maxWaitMs: 30000, pollMs: 250, initialMs: 150 };
  }
  const intervalMs = interval * 1000;
  return {
    interactive: false,
    maxWaitMs: Math.round(intervalMs * (1 + jitter / 100)) + 3000,
    pollMs: Math.min(5000, Math.max(800, Math.round(intervalMs * 0.2))),
    initialMs: Math.min(3000, Math.max(400, Math.round(intervalMs * 0.1))),
  };
}

export async function wakeAgentBeacon(agentId: string) {
  await fetch(`${API_BASE}?p=/agents/${agentId}/beacon_now&format=json`, {
    method: "POST",
    credentials: "include",
  }).catch((e) => console.error("wakeAgentBeacon failed", e));
}

export async function fetchAgentBeaconTiming(agentId: string): Promise<{ interval: number; jitter: number }> {
  try {
    const res = await fetch(`${API_BASE}?p=/toolkit/agents/${agentId}/info&format=json`, { credentials: "include" });
    if (!res.ok) return { interval: 10, jitter: 20 };
    const data = await res.json();
    const ag = data.agent || data.Agent || {};
    return {
      interval: Number(ag.current_interval ?? ag.CurrentInterval ?? 10),
      jitter: Number(ag.current_jitter ?? ag.CurrentJitter ?? 20),
    };
  } catch {
    return { interval: 10, jitter: 20 };
  }
}

export async function sendShellCommand(
  agentId: string,
  command: string,
  shell: string
): Promise<{ success: boolean; taskId?: number; error?: string }> {
  const body = new URLSearchParams({ command, shell });
  const res = await fetch(`${API_BASE}?p=/agents/${agentId}/command&format=json`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: body.toString(),
  });
  const data = await res.json();
  if (!res.ok || !data.success) {
    return { success: false, error: data.error || `HTTP ${res.status}` };
  }
  return { success: true, taskId: data.task_id || data.taskId };
}

export async function pollTaskResult(
  agentId: string,
  taskId: number,
  timing: ReturnType<typeof getShellTiming>,
  onWaiting?: () => void
): Promise<{ status: string; result?: string; error?: string }> {
  const started = Date.now();
  const poll = async (): Promise<{ status: string; result?: string; error?: string }> => {
    const res = await fetch(`${API_BASE}?p=/agents/${agentId}/tasks/${taskId}&format=json`, {
      credentials: "include",
    });
    const data = await res.json();
    if (data.error) return { status: "failed", error: data.error };

    const status = data.status || data.Status || "pending";
    if (status === "completed") {
      return { status, result: decodeShellResult(data) || "Command executed successfully" };
    }
    if (status === "failed") {
      return { status, error: data.error || "Command failed" };
    }
    if (Date.now() - started >= timing.maxWaitMs) {
      return { status: "timeout", error: "Timed out waiting for agent response" };
    }
    onWaiting?.();
    await new Promise((r) => setTimeout(r, timing.pollMs));
    return poll();
  };
  await new Promise((r) => setTimeout(r, timing.initialMs));
  return poll();
}

const HISTORY_KEY = "forgec2_shell_history";
const MAX_HISTORY = 100;

export function loadCommandHistory(): string[] {
  try {
    const raw = localStorage.getItem(HISTORY_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

export function saveCommandHistory(cmd: string) {
  const hist = loadCommandHistory().filter((c) => c !== cmd);
  hist.unshift(cmd);
  localStorage.setItem(HISTORY_KEY, JSON.stringify(hist.slice(0, MAX_HISTORY)));
}