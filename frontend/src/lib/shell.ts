import { api } from "./api";

export function decodeShellResult(data: { result?: string; encoding?: string }): string {
  const out = data?.result ? String(data.result) : "";
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
  localStorage.setItem(HISTORY_KEY, JSON.stringify(hist.slice(0, MAX_HISTORY)));
}
