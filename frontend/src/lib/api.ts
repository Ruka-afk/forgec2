"use client";

import { API_BASE } from "./constants";
import { onWSMessage } from "./wsContext";
import { paths } from "./api-paths";

const TIMEOUT_MS = 30000;

let rateLimitRetryAfter = 0;

export function getRateLimitRetryAfter(): number {
  return Math.max(0, rateLimitRetryAfter - Math.floor(Date.now() / 1000));
}

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

export function unwrapBody<T>(body: unknown): T {
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

let authRedirecting = false;
let authRedirectTimer: ReturnType<typeof setTimeout> | null = null;

/** Test-only: reset 401 redirect debounce state between cases. */
export function resetAuthRedirectState(): void {
  authRedirecting = false;
  if (authRedirectTimer) {
    clearTimeout(authRedirectTimer);
    authRedirectTimer = null;
  }
}

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export function handleUnauthorized(res: Pick<Response, "status">): void {
  // Only session expiry (401) forces re-login. 403 = authenticated but denied — stay put.
  if (res.status !== 401 || typeof window === "undefined") return;
  const p = window.location.pathname;
  if (p === "/login" || p === "/login/") return;
  if (authRedirecting) return;
  authRedirecting = true;

  // Defer so concurrent 401s collapse into one navigation
  if (authRedirectTimer) clearTimeout(authRedirectTimer);
  authRedirectTimer = setTimeout(() => {
    const next = p + (window.location.search || "");
    const params = new URLSearchParams();
    params.set("expired", "1");
    if (next && next !== "/" && next !== "/login") {
      params.set("next", next);
    }
    window.location.href = `/login?${params.toString()}`;
  }, 80);
}

interface RequestOptions {
  method?: string;
  retries?: number;
  timeout?: number;
  signal?: AbortSignal;
  headers?: Record<string, string>;
  /**
   * Skip unwrapBody: return the raw JSON envelope ({success, total, ...})
   * instead of the unwrapped `data` payload. Needed when the caller must see
   * pagination metadata (e.g. total) alongside the list.
   */
  unwrap?: boolean;
}

async function request<T>(path: string, options: RequestOptions & { body?: unknown; raw?: boolean } = {}): Promise<T> {
  const { retries = 0, timeout = TIMEOUT_MS, signal, headers: extraHeaders } = options;
  const method = options.method || "GET";
  const headers: Record<string, string> = {
    "Accept": "application/json",
    ...extraHeaders,
  };

  const isMutation = method !== "GET" && method !== "HEAD";
  const csrf = isMutation ? readCsrfCookie() : "";
  if (csrf) headers["X-CSRF-Token"] = csrf;
  const isFormData = options.body instanceof FormData;
  const isUrlEncoded = options.body instanceof URLSearchParams;
  const isString = typeof options.body === "string";

  if (!isFormData && !isUrlEncoded && isString && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  } else if (isUrlEncoded && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/x-www-form-urlencoded";
  }

  let body: BodyInit | undefined;
  if (isFormData) {
    body = options.body as FormData;
  } else if (isUrlEncoded) {
    body = options.body as URLSearchParams;
  } else if (isString) {
    body = options.body as string;
  } else if (options.body && method !== "GET" && method !== "HEAD") {
    body = JSON.stringify(options.body);
    if (!headers["Content-Type"]) headers["Content-Type"] = "application/json";
  }

  const doFetch = async (attempt: number): Promise<T> => {
    try {
      const res = await withTimeout(fetch(buildUrl(path), {
        method,
        credentials: "include",
        headers,
        body,
        signal,
      }), timeout);

      if (!res.ok) {
        handleUnauthorized(res);
        if (res.status === 429) {
          const retryAfter = res.headers.get("Retry-After");
          if (retryAfter) {
            rateLimitRetryAfter = Math.floor(Date.now() / 1000) + parseInt(retryAfter, 10);
          }
        }
        let errorMsg: string;
        if (res.status === 403) {
          errorMsg = isMutation && !csrf ? "CSRF token missing - refresh the page and try again" : "Forbidden";
        } else {
          errorMsg = `HTTP ${res.status}`;
        }
        try {
          const errBody = await res.json();
          if (errBody && typeof errBody === "object" && "error" in errBody) {
            errorMsg = String(errBody.error);
          }
        } catch { /* ignore parse error */ }
        throw new ApiError(errorMsg, res.status);
      }

      if (options.raw) {
        return res as unknown as T;
      }

      const respBody = await res.json();
      return options.unwrap === false ? (respBody as T) : unwrapBody<T>(respBody);
    } catch (e) {
      if (process.env.NODE_ENV === "development") console.error("api request failed", path, e);
      if (attempt >= retries) throw e;
      return new Promise<T>((resolve) =>
        setTimeout(() => resolve(doFetch(attempt + 1)), 800 * Math.pow(2, attempt))
      );
    }
  };

  return doFetch(0);
}

function parseFilenameFromDisposition(cd: string | null): string {
  if (!cd) return "download.bin";
  const star = cd.match(/filename\*\s*=\s*(?:UTF-8'')?([^;]+)/i);
  if (star) {
    try {
      const decoded = decodeURIComponent(star[1].trim().replace(/"/g, ""));
      if (decoded) return decoded;
    } catch { /* fall back to plain filename */ }
  }
  const plain = cd.match(/filename\s*=\s*"?([^";]+)"?/i);
  return plain ? plain[1].trim() : "download.bin";
}

export const api = {
  get<T = Record<string, unknown>>(path: string, opts?: { retries?: number; signal?: AbortSignal; unwrap?: boolean }): Promise<T> {
    return request<T>(path, { method: "GET", retries: opts?.retries ?? 0, signal: opts?.signal, unwrap: opts?.unwrap });
  },

  post<T = Record<string, unknown>>(path: string, data?: Record<string, string>, opts?: { signal?: AbortSignal }): Promise<T> {
    const body = data ? new URLSearchParams(data).toString() : undefined;
    return request<T>(path, {
      method: "POST",
      headers: body ? { "Content-Type": "application/x-www-form-urlencoded" } : {},
      body,
      signal: opts?.signal,
    });
  },

  postJson<T = Record<string, unknown>>(path: string, body: unknown, opts?: { signal?: AbortSignal }): Promise<T> {
    return request<T>(path, { method: "POST", body, signal: opts?.signal });
  },

  postFormData<T = Record<string, unknown>>(path: string, body: FormData): Promise<T> {
    return request<T>(path, { method: "POST", body, headers: {} });
  },

  put<T = Record<string, unknown>>(path: string, data?: Record<string, string>): Promise<T> {
    const body = data ? new URLSearchParams(data).toString() : undefined;
    return request<T>(path, {
      method: "PUT",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body,
    });
  },

  putJson<T = Record<string, unknown>>(path: string, body: unknown): Promise<T> {
    return request<T>(path, { method: "PUT", body });
  },

  del<T = Record<string, unknown>>(path: string): Promise<T> {
    return request<T>(path, { method: "DELETE" });
  },

  async download(path: string, data?: Record<string, string>): Promise<{ blob: Blob; filename: string }> {
    const body = data ? new URLSearchParams(data).toString() : undefined;
    const res = await request<Response>(path, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body,
      raw: true,
    });
    return { blob: await res.blob(), filename: parseFilenameFromDisposition(res.headers.get("Content-Disposition")) };
  },

  async downloadGet(path: string): Promise<{ blob: Blob; filename: string }> {
    const res = await request<Response>(path, {
      method: "GET",
      raw: true,
    });
    return { blob: await res.blob(), filename: parseFilenameFromDisposition(res.headers.get("Content-Disposition")) };
  },
};

export { buildUrl };

type TaskStatusBase = {
  id: number;
  result?: string;
  error?: string;
  command?: string;
  type?: string;
  agent?: string;
  created?: string;
};

type TaskRunning = TaskStatusBase & { status: "running" | "pending" };
type TaskCompleted = TaskStatusBase & { status: "completed"; result: string };
type TaskFailed = TaskStatusBase & { status: "failed"; error: string };
export type TaskStatus = TaskRunning | TaskCompleted | TaskFailed;

export interface PollTaskHandle {
  promise: Promise<TaskStatus>;
  cancel: () => void;
}

export async function pollTask(
  agentId: string,
  taskId: number,
  opts: { intervalMs?: number; timeoutMs?: number; signal?: AbortSignal; onStatus?: (st: TaskStatus) => void } = {},
): Promise<TaskStatus> {
  const h = pollTaskWithCancel(agentId, taskId, opts);
  return h.promise;
}

function pollTaskWithCancel(
  agentId: string,
  taskId: number,
  opts: { intervalMs?: number; timeoutMs?: number; signal?: AbortSignal; onStatus?: (st: TaskStatus) => void } = {},
): PollTaskHandle {
  const intervalMs = opts.intervalMs ?? 1500;
  const timeoutMs = opts.timeoutMs ?? 60000;
  const deadline = Date.now() + timeoutMs;
  const ac = new AbortController();
  const signal = opts.signal ?? ac.signal;

  let done = false;
  let unsub: (() => void) | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let rejectRef: ((err: Error) => void) | null = null;

  const cleanup = () => {
    done = true;
    if (unsub) unsub();
    if (timer) clearTimeout(timer);
    if (!opts.signal) ac.abort();
  };

  const finalFetch = async (fallback: TaskStatus): Promise<TaskStatus> => {
    try {
      return await api.get<TaskStatus>(paths.agents.task(agentId, taskId));
    } catch {
      return fallback;
    }
  };

  const promise = new Promise<TaskStatus>((resolve, reject) => {
    rejectRef = reject;
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

    try {
      let streamedOutput = "";
      unsub = onWSMessage((msg) => {
        if (msg.type === "task_output" && Number(msg.task_id) === taskId) {
          // Ordered streaming frame for large shell results: deliver the
          // accumulated output so callers can render incrementally.
          streamedOutput += String(msg.chunk ?? "");
          opts.onStatus?.({
            id: taskId,
            status: "running",
            result: streamedOutput,
          } as TaskStatus);
          if (msg.done) {
            finish({
              id: taskId,
              status: "completed",
              result: streamedOutput,
            } as TaskStatus);
          }
          return;
        }
        if (msg.type !== "task_update" || Number(msg.task_id) !== taskId) return;
        const status = String(msg.status) as TaskStatus["status"];
        const partial = {
          id: taskId,
          status,
          result: msg.result as string | undefined,
          error: msg.error as string | undefined,
        } as TaskStatus;
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
        const st = await api.get<TaskStatus>(paths.agents.task(agentId, taskId));
        opts.onStatus?.(st);
        if (st.status === "completed" || st.status === "failed") return finish(st);
      } catch (err) {
        if (process.env.NODE_ENV === "development") console.error("[API:pollTask]", err);
      }
      timer = setTimeout(tick, intervalMs);
    };
    tick();
  });

  return {
    promise,
    cancel: () => {
      if (done) return;
      cleanup();
      rejectRef?.(new Error("cancelled"));
    },
  };
}

import type { AgentStatus } from "@/types/agent";

const VALID_STATUSES: readonly string[] = ["online", "stale", "offline"];

async function getAgentStatus(agentId: string): Promise<AgentStatus | "unknown"> {
  const data = await api.get<{
    Agent?: { status?: string };
    status?: string;
    data?: { status?: string };
  }>(paths.agents.one(agentId));
  const agent = data.data || data;
  const raw = agent?.status || data.status || "unknown";
  return (VALID_STATUSES.includes(raw) ? raw : "unknown") as AgentStatus | "unknown";
}

export interface RunTaskOptions {
  method?: "post" | "postJson";
  body?: Record<string, unknown> | Record<string, string>;
  intervalMs?: number;
  timeoutMs?: number;
  checkOnline?: boolean;
  onStatus?: (st: TaskStatus) => void;
  signal?: AbortSignal;
}

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


