import { api } from "./api";

export interface FileEntry {
  name: string;
  is_dir: boolean;
  size: number;
  mod_time: string;
}

export function parseLsResult(text: string): FileEntry[] {
  const lines = text.split("\n").map((l) => l.trim()).filter(Boolean);
  const entries: FileEntry[] = [];
  let headerPassed = false;
  for (const line of lines) {
    if (!headerPassed) {
      if (line.startsWith("---")) { headerPassed = true; }
      continue;
    }
    const parts = line.split("\t");
    if (parts.length < 4) continue;
    const type = parts[0].trim().toUpperCase();
    entries.push({
      name: parts[1]?.trim() || "",
      is_dir: type === "DIR",
      size: type === "DIR" ? 0 : Number(parts[2]?.trim()) || 0,
      mod_time: parts[3]?.trim() || "",
    });
  }
  return entries;
}

export async function sendFileCommand(
  agentId: string,
  endpoint: string,
  params: Record<string, string>
): Promise<{ success: boolean; taskId?: number; error?: string }> {
  try {
    const data = await api.post<{ success?: boolean; error?: string; task_id?: number; taskId?: number }>(`/agents/${agentId}${endpoint}`, params);
    if (!data.success) {
      return { success: false, error: data.error || "Command failed" };
    }
    return { success: true, taskId: data.task_id ?? data.taskId };
  } catch (e) {
    return { success: false, error: String(e) };
  }
}

export async function pollFileTaskResult(
  agentId: string,
  taskId: number,
  maxWaitMs = 60000,
  pollMs = 1000,
  signal?: AbortSignal
): Promise<{ status: string; result?: string; error?: string }> {
  const started = Date.now();
  const poll = async (): Promise<{ status: string; result?: string; error?: string }> => {
    if (signal?.aborted) return { status: "cancelled", error: "Cancelled" };
    const data = await api.get<{ error?: string; status?: string; Status?: string; result?: string }>(`/agents/${agentId}/tasks/${taskId}`);
    if (data.error) return { status: "failed", error: data.error };
    const status = data.status || "pending";
    if (status === "completed") {
      return { status, result: data.result || "" };
    }
    if (status === "failed") {
      return { status, error: data.error || "Task failed" };
    }
    if (Date.now() - started >= maxWaitMs) {
      return { status: "timeout", error: "Timed out waiting for agent" };
    }
    await new Promise((r) => setTimeout(r, pollMs));
    return poll();
  };
  await new Promise((r) => setTimeout(r, Math.min(500, pollMs)));
  return poll();
}

export async function sendAndPollFileTask(
  agentId: string,
  endpoint: string,
  params: Record<string, string>,
  maxWaitMs?: number
): Promise<{ success: boolean; result?: string; error?: string }> {
  const cmd = await sendFileCommand(agentId, endpoint, params);
  if (!cmd.success) return { success: false, error: cmd.error };
  const result = await pollFileTaskResult(agentId, cmd.taskId!, maxWaitMs);
  if (result.status === "completed") {
    return { success: true, result: result.result };
  }
  return { success: false, error: result.error || "Task failed" };
}
