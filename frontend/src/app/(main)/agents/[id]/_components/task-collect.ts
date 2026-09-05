import { api, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";

/**
 * Dispatches an agent task via POST and waits for its result: polls until
 * terminal, then fetches the full task record. Returns the result string
 * ("" when the task produced no output).
 */
export async function collectTaskResult(
  agentId: string,
  path: string,
  body: Record<string, string> = {},
  timeoutMs = 120_000,
): Promise<string> {
  const dispatched = (await api.post(path, body)) as { task_id?: number };
  const taskId = dispatched.task_id;
  if (!taskId) throw new Error("dispatch failed: no task id");
  const st = await pollTask(agentId, taskId, { timeoutMs });
  if (st.status === "failed") throw new Error(st.error || "task failed");
  const task = (await api.get(paths.agents.task(agentId, String(taskId)))) as {
    result?: string;
    data?: { result?: string };
  };
  return task.result ?? task.data?.result ?? "";
}

/** Decodes a base64 task result to bytes for binary downloads. */
export function base64ToBytes(b64: string): Uint8Array<ArrayBuffer> {
  const compact = b64.replace(/\s+/g, "");
  const bin = atob(compact);
  const out = new Uint8Array(new ArrayBuffer(bin.length));
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}
