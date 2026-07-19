"use client";

import { API_BASE } from "./constants";
import { onWSMessage } from "./wsContext";

const TIMEOUT_MS = 30000;

function readCsrfCookie(): string {
  if (typeof document === "undefined") return "";
  const match = document.cookie.match(/(?:^|;\s*)forgec2_csrf=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : "";
}

export function getCsrfToken(): string {
  return readCsrfCookie();
}

function buildUrl(path: string): string {
  return `${API_BASE}${path}`;
}

// unwrapBody: if the server returned the standard { success, data } envelope,
// return the inner `data` payload; otherwise return the raw body unchanged.
// This lets the backend migrate to the envelope incrementally without breaking
// existing callers that read bare keys.
function unwrapBody<T>(body: unknown): T {
  if (body && typeof body === "object" && "success" in body && (body as Record<string, unknown>).success === true && "data" in body) {
    return (body as Record<string, unknown>).data as T;
  }
  return body as T;
}

function withTimeout<T>(promise: Promise<T>, ms: number): Promise<T> {
  let timer: ReturnType<typeof setTimeout>;
  return Promise.race([
    promise.finally(() => clearTimeout(timer)),
    new Promise<never>((_, reject) => {
      timer = setTimeout(() => reject(new Error(`Request timed out after ${ms}ms`)), ms);
    }),
  ]);
}

function handleUnauthorized(res: Response): void {
  if (res.status === 401 && typeof window !== "undefined") {
    window.location.href = "/login";
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const method = options.method || "GET";
  const headers: Record<string, string> = {
    "Accept": "application/json",
    ...(options.headers as Record<string, string>),
  };
  if (!headers["Content-Type"] && !(options.body instanceof FormData) && !(options.body instanceof URLSearchParams) && typeof options.body === "string") {
    headers["Content-Type"] = "application/json";
  }
  if (method !== "GET" && method !== "HEAD") {
    const csrf = readCsrfCookie();
    if (csrf) headers["X-CSRF-Token"] = csrf;
  }
  try {
    const res = await withTimeout(fetch(buildUrl(path), { ...options, headers, credentials: "include" }), TIMEOUT_MS);
    if (!res.ok) { handleUnauthorized(res); throw new Error(`HTTP ${res.status}`); }
    const body = await res.json();
    return unwrapBody<T>(body);
  } catch (e) {
    if (process.env.NODE_ENV === "development") console.error("api request failed", path, e);
    throw e;
  }
}


export const api = {
  get<T = Record<string, unknown>>(path: string, retries = 0): Promise<T> {
    const doFetch = (attempt: number): Promise<T> =>
      withTimeout(fetch(buildUrl(path), { credentials: "include", headers: { "Accept": "application/json" } }), TIMEOUT_MS)
        .then(async (res) => {
          if (!res.ok) { handleUnauthorized(res); throw new Error(`HTTP ${res.status}`); }
          const body = await res.json();
          return unwrapBody<T>(body);
        })
        .catch((e) => {
          if (process.env.NODE_ENV === "development") console.error("api.get failed", path, e);
          if (attempt >= retries) throw e;
          return new Promise<T>((resolve) =>
            setTimeout(() => resolve(doFetch(attempt + 1)), 800 * (attempt + 1))
          );
        });
    return doFetch(0);
  },

  post<T = Record<string, unknown>>(path: string, data?: Record<string, string>): Promise<T> {
    return request<T>(path, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: data ? new URLSearchParams(data).toString() : undefined,
    });
  },

  postJson<T = Record<string, unknown>>(path: string, body: unknown): Promise<T> {
    return request<T>(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  },

  async postFormData<T = Record<string, unknown>>(path: string, body: FormData): Promise<T> {
    const headers: Record<string, string> = {};
    const csrf = readCsrfCookie();
    if (csrf) headers["X-CSRF-Token"] = csrf;
    try {
      const res = await withTimeout(fetch(buildUrl(path), {
        method: "POST",
        credentials: "include",
        headers,
        body,
      }), TIMEOUT_MS);
      if (!res.ok) { handleUnauthorized(res); throw new Error(`HTTP ${res.status}`); }
      const respBody = await res.json();
      return unwrapBody<T>(respBody);
    } catch (e) {
      if (process.env.NODE_ENV === "development") console.error("api.postFormData failed", path, e);
      throw e;
    }
  },

  put<T = Record<string, unknown>>(path: string, data?: Record<string, string>): Promise<T> {
    return request<T>(path, {
      method: "PUT",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: data ? new URLSearchParams(data).toString() : undefined,
    });
  },

  putJson<T = Record<string, unknown>>(path: string, body: unknown): Promise<T> {
    return request<T>(path, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  },

  del<T = Record<string, unknown>>(path: string): Promise<T> {
    return request<T>(path, { method: "DELETE" });
  },

  // json<T> calls an envelope endpoint and returns the UNWRAPPED `data`
  // payload (the server's { success, data } envelope is stripped here), so
  // callers never write `resp.data?.x`. Only use it on endpoints that return
  // the envelope; bare-shape endpoints should keep using api.get.
  json<T = Record<string, unknown>>(path: string, retries = 0): Promise<T> {
    return api.get<T>(path, retries);
  },

  async download(path: string, data?: Record<string, string>): Promise<{ blob: Blob; filename: string }> {
    const body = data ? new URLSearchParams(data).toString() : undefined;
    const dlHeaders: Record<string, string> = { "Content-Type": "application/x-www-form-urlencoded" };
    const csrf = readCsrfCookie();
    if (csrf) dlHeaders["X-CSRF-Token"] = csrf;
    const res = await withTimeout(fetch(buildUrl(path), {
      method: "POST",
      credentials: "include",
      headers: dlHeaders,
      body,
    }), TIMEOUT_MS);
    if (!res.ok) { handleUnauthorized(res); throw new Error(`HTTP ${res.status}`); }
    const cd = res.headers.get("Content-Disposition");
    let filename = "download.bin";
    if (cd) {
      const m = cd.match(/filename=(.+)/);
      if (m) filename = m[1].replace(/"/g, "");
    }
    return { blob: await res.blob(), filename };
  },
};

export { buildUrl };

// TaskStatus mirrors the JSON returned by GET /agents/:id/tasks/:taskId
// (handleGetTaskStatus). `result` holds the agent's stdout / listing JSON;
// `status` is one of pending | running | completed | failed.
export interface TaskStatus {
  id: number;
  status: string;
  result?: string;
  error?: string;
  command?: string;
  type?: string;
  agent?: string;
  created?: string;
}

// pollTask waits for an async agent task (shell, file listing, etc.) to finish
// by repeatedly querying GET /agents/:id/tasks/:taskId, and ALSO subscribes to
// the WebSocket: when the server broadcasts a `task_update` for this task_id the
// wait ends immediately (real-time), then a final HTTP fetch returns the FULL
// result (the WS copy is truncated to 200 chars). C2 is asynchronous: the
// operator's request only *queues* the task; the agent returns the result on its
// next beacon, stored in Task.Result. Throws if it does not complete within
// timeoutMs (e.g. the agent is offline).
export async function pollTask(
  agentId: string,
  taskId: number,
  opts: { intervalMs?: number; timeoutMs?: number; signal?: AbortSignal; onStatus?: (st: TaskStatus) => void } = {},
): Promise<TaskStatus> {
  const intervalMs = opts.intervalMs ?? 1500;
  const timeoutMs = opts.timeoutMs ?? 60000;
  const deadline = Date.now() + timeoutMs;
  const ac = new AbortController();
  const signal = opts.signal ?? ac.signal;

  return new Promise<TaskStatus>((resolve, reject) => {
    let done = false;
    let unsub: (() => void) | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const cleanup = () => {
      done = true;
      if (unsub) unsub();
      if (timer) clearTimeout(timer);
      if (!opts.signal) ac.abort();
    };

    const finalFetch = async (fallback: TaskStatus): Promise<TaskStatus> => {
      try {
        return await api.get<TaskStatus>(`/agents/${agentId}/tasks/${taskId}`);
      } catch {
        return fallback;
      }
    };

    const finish = async (st: TaskStatus) => {
      if (done) return;
      cleanup();
      resolve(await finalFetch(st));
    };
    const fail = (err: Error) => {
      if (done) return;
      cleanup();
      reject(err);
    };

    // Real-time completion via WebSocket (no hook needed).
    try {
      unsub = onWSMessage((msg) => {
        if (msg.type !== "task_update" || Number(msg.task_id) !== taskId) return;
        const status = String(msg.status);
        const partial: TaskStatus = {
          id: taskId,
          status,
          result: msg.result as string | undefined,
          error: msg.error as string | undefined,
        };
        opts.onStatus?.(partial);
        if (status === "completed" || status === "failed") finish(partial);
      });
    } catch {
      unsub = null;
    }

    const tick = async () => {
      if (done) return;
      if (signal.aborted) return fail(new Error("cancelled"));
      if (Date.now() > deadline) return fail(new Error("Agent did not respond within the timeout (is it online?)"));
      try {
        const st = await api.get<TaskStatus>(`/agents/${agentId}/tasks/${taskId}`);
        opts.onStatus?.(st);
        if (st.status === "completed" || st.status === "failed") return finish(st);
      } catch (err) {
        if (process.env.NODE_ENV === "development") console.error("[API:pollTask]", err);
      }
      timer = setTimeout(tick, intervalMs);
    };
    tick();
  });
}

// getAgentStatus returns the live status of an agent ("online" | "offline" |
// "stale" | ...). Used to give a clear error before queuing a task against an
// agent that is not connected.
export async function getAgentStatus(agentId: string): Promise<string> {
  const data = await api.get<{
    Agent?: { status?: string };
    status?: string;
    data?: { status?: string };
  }>(`/api/v1/agents/${agentId}`);
  const agent = data.data || data;
  return (agent?.status || data.status || "unknown") as string;
}

export interface RunTaskOptions {
  // "post" sends form-urlencoded; "postJson" sends a JSON body.
  method?: "post" | "postJson";
  body?: Record<string, unknown> | Record<string, string>;
  intervalMs?: number;
  timeoutMs?: number;
  // When true, refuse to queue if the agent is not "online" (clear error).
  checkOnline?: boolean;
  // Called on every poll/WS status update (e.g. to show "waiting for agent").
  onStatus?: (st: TaskStatus) => void;
  signal?: AbortSignal;
}

// runTask is the single entry point for interactive, async agent features
// (shell, file browser, etc.). It queues the task, waits for the agent's result
// via pollTask (HTTP + WebSocket), and returns the final TaskStatus. Centralizing
// this prevents the class of bug where a feature reads the immediate
// queue-acknowledgement response (which has no output) instead of the result.
export async function runTask(
  agentId: string,
  path: string,
  opts: RunTaskOptions = {},
): Promise<TaskStatus> {
  if (opts.checkOnline) {
    const status = await getAgentStatus(agentId).catch(() => "unknown");
    if (status !== "online") {
      throw new Error(`Agent is ${status} — start the agent and ensure it is online before running this.`);
    }
  }
  const postFn = opts.method === "postJson" ? api.postJson : api.post;
  const res = await postFn<{ success?: boolean; task_id?: number; error?: string }>(
    path,
    (opts.body as Record<string, string>) || {},
  );
  const taskId = res.task_id;
  if (!taskId) {
    const errMsg = (res as Record<string, unknown>).error;
    throw new Error(typeof errMsg === "string" ? errMsg : "Failed to queue task (no task_id returned)");
  }
  return pollTask(agentId, taskId, {
    intervalMs: opts.intervalMs,
    timeoutMs: opts.timeoutMs,
    signal: opts.signal,
    onStatus: opts.onStatus,
  });
}

