"use client";

import { API_BASE } from "./constants";

const TIMEOUT_MS = 30000;

interface ApiResponse<T> {
  success?: boolean;
  error?: string;
  data?: T;
}

function buildUrl(path: string): string {
  return `${API_BASE}?p=${encodeURIComponent(path)}`;
}

function withTimeout<T>(promise: Promise<T>, ms: number): Promise<T> {
  return Promise.race([
    promise,
    new Promise<never>((_, reject) => setTimeout(() => reject(new Error(`Request timed out after ${ms}ms`)), ms)),
  ]);
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/x-www-form-urlencoded",
    ...(options.headers as Record<string, string>),
  };
  const res = await withTimeout(fetch(buildUrl(path), { ...options, headers, credentials: "include" }), TIMEOUT_MS);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

async function requestBlob(path: string, body?: BodyInit): Promise<Blob> {
  const res = await withTimeout(fetch(buildUrl(path), {
    method: "POST",
    credentials: "include",
    body,
  }), TIMEOUT_MS);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.blob();
}

export const api = {
  get<T = Record<string, unknown>>(path: string, retries = 0): Promise<T> {
    const doFetch = (attempt: number): Promise<T> =>
      withTimeout(fetch(buildUrl(path), { credentials: "include" }), TIMEOUT_MS)
        .then(async (res) => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`);
          return res.json();
        })
        .catch((e) => {
          if (attempt >= retries) throw e;
          return new Promise<T>((resolve) =>
            setTimeout(() => resolve(doFetch(attempt + 1)), 800 * (attempt + 1))
          );
        });
    return doFetch(0);
  },

  post<T = ApiResponse<unknown>>(path: string, data?: Record<string, string>): Promise<T> {
    return request<T>(path, {
      method: "POST",
      body: data ? new URLSearchParams(data).toString() : undefined,
    });
  },

  postJson<T = ApiResponse<unknown>>(path: string, body: unknown): Promise<T> {
    return withTimeout(fetch(buildUrl(path), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(body),
    }), TIMEOUT_MS).then(async (res) => {
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    }) as Promise<T>;
  },

  put<T = ApiResponse<unknown>>(path: string, data?: Record<string, string>): Promise<T> {
    return request<T>(path, {
      method: "PUT",
      body: data ? new URLSearchParams(data).toString() : undefined,
    });
  },

  del<T = ApiResponse<unknown>>(path: string): Promise<T> {
    return request<T>(path, { method: "DELETE" });
  },

  async download(path: string, data?: Record<string, string>): Promise<{ blob: Blob; filename: string }> {
    const body = data ? new URLSearchParams(data).toString() : undefined;
    const res = await withTimeout(fetch(buildUrl(path), {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body,
    }), TIMEOUT_MS);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
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

export async function apiGet<T = Record<string, unknown>>(path: string, retries = 0): Promise<T> {
  return api.get<T>(path, retries);
}
export async function apiSend(path: string, method: string, data?: URLSearchParams) {
  return api[method.toLowerCase() === "post" ? "post" : "put"](path, Object.fromEntries(data?.entries() ?? []));
}
export async function apiPostJson<T = Record<string, unknown>>(path: string, body: unknown): Promise<T> {
  return api.postJson<T>(path, body);
}
export async function apiPut(path: string, data?: URLSearchParams) {
  return api.put(path, Object.fromEntries(data?.entries() ?? []));
}
export async function apiDelete(path: string) {
  return api.del(path);
}
