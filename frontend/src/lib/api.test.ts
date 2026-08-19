import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  getCsrfToken,
  unwrapBody,
  buildUrl,
  api,
  ApiError,
  handleUnauthorized,
  resetAuthRedirectState,
  getRateLimitRetryAfter,
} from "./api";

describe("unwrapBody", () => {
  it("extracts data from success envelope", () => {
    const result = unwrapBody<{ id: number }>({ success: true, data: { id: 42 } });
    expect(result).toEqual({ id: 42 });
  });

  it("returns body as-is when success is false", () => {
    const body = { success: false, error: "fail" };
    expect(unwrapBody(body)).toBe(body);
  });

  it("returns body as-is when success is missing", () => {
    const body = { error: "fail" };
    expect(unwrapBody(body)).toBe(body);
  });

  it("returns body as-is when data is missing", () => {
    const body = { success: true };
    expect(unwrapBody(body)).toBe(body);
  });

  it("returns non-object values as-is", () => {
    expect(unwrapBody("hello")).toBe("hello");
    expect(unwrapBody(42)).toBe(42);
    expect(unwrapBody(null)).toBeNull();
    expect(unwrapBody(undefined)).toBeUndefined();
  });
});

describe("getCsrfToken", () => {
  it("returns empty string when no cookie", () => {
    document.cookie = "forgec2_csrf=; Max-Age=0";
    expect(getCsrfToken()).toBe("");
  });

  it("reads forgec2_csrf cookie", () => {
    document.cookie = "forgec2_csrf=abc123";
    expect(getCsrfToken()).toBe("abc123");
  });
});

describe("buildUrl", () => {
  it("returns the path unchanged with empty API_BASE", () => {
    expect(buildUrl("/api/tags")).toBe("/api/tags");
  });
});

describe("handleUnauthorized", () => {
  beforeEach(() => {
    resetAuthRedirectState();
    vi.useFakeTimers();
  });

  afterEach(() => {
    resetAuthRedirectState();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("does nothing for non-401", () => {
    const href = window.location.href;
    handleUnauthorized({ status: 403 });
    vi.advanceTimersByTime(100);
    expect(window.location.href).toBe(href);
  });

  it("skips redirect when already on login", () => {
    vi.stubGlobal("location", {
      ...window.location,
      pathname: "/login",
      search: "",
      href: "http://localhost/login",
    });
    const assign = vi.fn();
    Object.defineProperty(window.location, "href", {
      configurable: true,
      get: () => "http://localhost/login",
      set: assign,
    });
    handleUnauthorized({ status: 401 });
    vi.advanceTimersByTime(100);
    expect(assign).not.toHaveBeenCalled();
  });

  it("debounces concurrent 401s into one navigation with next", () => {
    const hrefs: string[] = [];
    const loc = {
      pathname: "/agents/abc/shell",
      search: "?tab=1",
      _href: "http://localhost/agents/abc/shell?tab=1",
      get href() {
        return this._href;
      },
      set href(v: string) {
        hrefs.push(v);
        this._href = v;
      },
    };
    vi.stubGlobal("location", loc);
    // handleUnauthorized uses window.location
    Object.defineProperty(window, "location", { configurable: true, value: loc, writable: true });

    handleUnauthorized({ status: 401 });
    handleUnauthorized({ status: 401 });
    vi.advanceTimersByTime(100);

    expect(hrefs.length).toBe(1);
    expect(hrefs[0]).toContain("/login?");
    expect(hrefs[0]).toContain("expired=1");
    expect(hrefs[0]).toMatch(/next=%2Fagents%2Fabc%2Fshell/);
  });
});

describe("api.get network layer", () => {
  beforeEach(() => {
    resetAuthRedirectState();
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetAuthRedirectState();
  });

  it("unwraps success envelope", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ success: true, data: { ok: 1 } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const data = await api.get<{ ok: number }>("/agents");
    expect(data).toEqual({ ok: 1 });
  });

  it("throws ApiError with status on 403 without redirect", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "forbidden" }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      }),
    );
    try {
      await api.get("/secret");
      expect.fail("should throw");
    } catch (e) {
      expect(e).toBeInstanceOf(ApiError);
      expect((e as ApiError).status).toBe(403);
      expect((e as ApiError).message).toBe("forbidden");
    }
  });

  it("records Retry-After on 429", async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "rate limited" }), {
        status: 429,
        headers: { "Content-Type": "application/json", "Retry-After": "30" },
      }),
    );
    await expect(api.get("/busy")).rejects.toThrow(/rate limited|HTTP 429/);
    expect(getRateLimitRetryAfter()).toBeGreaterThan(0);
  });

  it("sends CSRF header on POST when cookie present", async () => {
    document.cookie = "forgec2_csrf=tok-csrf";
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ success: true, data: {} }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    await api.post("/settings", { a: "1" });
    expect(fetch).toHaveBeenCalled();
    const init = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
    const headers = init.headers as Record<string, string>;
    expect(headers["X-CSRF-Token"]).toBe("tok-csrf");
  });

  it("aborts the underlying fetch on timeout", async () => {
    let capturedSignal: AbortSignal | null = null;
    vi.mocked(fetch).mockImplementation((_input, init) => {
      capturedSignal = init?.signal ?? null;
      return new Promise<Response>(() => { /* never settles */ });
    });
    const t0 = Date.now();
    try {
      await api.get<unknown>("/hang", { retries: 0, timeout: 50 });
      expect.fail("should throw on timeout");
    } catch (e) {
      expect(e).toBeInstanceOf(Error);
      expect((e as Error).message).toMatch(/timed out after/);
    }
    expect(Date.now() - t0).toBeLessThan(2000);
    expect(capturedSignal).not.toBeNull();
    expect(capturedSignal!.aborted).toBe(true);
  });

  it("deduplicates concurrent identical GET requests", async () => {
    let resolveFirst: (r: Response) => void = () => {};
    const fetchMock = vi.fn(() => new Promise<Response>((resolve) => { resolveFirst = resolve; }));
    vi.stubGlobal("fetch", fetchMock);

    const p1 = api.get("/agents");
    const p2 = api.get("/agents");
    expect(fetchMock).toHaveBeenCalledTimes(1);

    resolveFirst(new Response(JSON.stringify({ success: true, data: { agents: [] } }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));
    const [r1, r2] = await Promise.all([p1, p2]);
    expect(r1).toEqual({ agents: [] });
    expect(r2).toEqual({ agents: [] });

    // Cache clears after settle: a later request fires a fresh fetch.
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ success: true, data: { agents: ["x"] } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    await api.get("/agents");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("does not dedup GETs with a caller-supplied signal", async () => {
    vi.mocked(fetch).mockImplementation(() =>
      Promise.resolve(new Response(JSON.stringify({ success: true, data: {} }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })),
    );
    const ac = new AbortController();
    const p1 = api.get("/agents", { signal: ac.signal });
    const p2 = api.get("/agents", { signal: ac.signal });
    await Promise.all([p1, p2]);
    expect(fetch).toHaveBeenCalledTimes(2);
  });
});

describe("api.download filename parsing", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("prefers UTF-8 filename* over plain filename", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      new Response(new Blob(["x"]), {
        status: 200,
        headers: {
          "Content-Type": "application/octet-stream",
          "Content-Disposition": "attachment; filename=\"fallback.bin\"; filename*=UTF-8''screenshot%20%E6%88%AA%E5%9B%BE.png",
        },
      }),
    ));
    const { filename } = await api.downloadGet("/dl");
    expect(filename).toBe("screenshot 截图.png");
  });

  it("falls back to plain quoted filename when no filename*", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      new Response(new Blob(["x"]), {
        status: 200,
        headers: {
          "Content-Type": "application/octet-stream",
          "Content-Disposition": "attachment; filename=\"build.exe\"",
        },
      }),
    ));
    const { filename } = await api.downloadGet("/dl");
    expect(filename).toBe("build.exe");
  });

  it("defaults to download.bin when no Content-Disposition", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      new Response(new Blob(["x"]), { status: 200 }),
    ));
    const { filename } = await api.downloadGet("/dl");
    expect(filename).toBe("download.bin");
  });
});
