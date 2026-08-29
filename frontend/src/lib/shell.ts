import { api } from "./api";

export async function fetchAgentBeaconTiming(agentId: string): Promise<{ interval: number; jitter: number }> {
  try {
    const data = await api.get<{ agent?: Record<string, unknown>; Agent?: Record<string, unknown> }>(`/toolkit/agents/${agentId}/info?format=json`);
    const ag = (data.agent || {}) as Record<string, unknown>;
    return {
      interval: Number(ag.current_interval ?? 10),
      jitter: Number(ag.current_jitter ?? 20),
    };
  } catch {
    return { interval: 10, jitter: 20 };
  }
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
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(hist.slice(0, MAX_HISTORY)));
  } catch {
    // Storage blocked/quota-exceeded (Safari lockdown, webviews): history is
    // best-effort — never let it break command execution.
  }
}
