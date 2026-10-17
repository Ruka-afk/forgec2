import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useRemoteDesktop } from "./useRemoteDesktop";
import { api } from "@/lib/api";

type WSMessage = Record<string, unknown>;

const hooksMock = vi.hoisted(() => ({ cb: null as null | (() => void), delay: 0 }));
const wsMock = vi.hoisted(() => ({ handler: null as null | ((msg: WSMessage) => void) }));
const toastMock = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));

vi.mock("@/lib/hooks/useVisibleInterval", () => ({
  useVisibleInterval: (cb: () => void, delay: number) => {
    hooksMock.cb = cb;
    hooksMock.delay = delay;
  },
}));

vi.mock("@/lib/wsContext", () => ({
  useWS: () => ({
    subscribe: (handler: (msg: WSMessage) => void) => {
      wsMock.handler = handler;
      return () => {
        wsMock.handler = null;
      };
    },
  }),
}));

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn(), postJson: vi.fn() },
}));

vi.mock("@/lib/i18n", () => ({
  getLocale: () => "en",
  useI18n: () => ({ t: (key: string) => key }),
}));

vi.mock("sonner", () => ({
  toast: { success: toastMock.success, error: toastMock.error },
}));

const mockedGet = vi.mocked(api.get);
const mockedPost = vi.mocked(api.post);

function screenshotCalls(): number {
  return mockedGet.mock.calls.filter((call) => String(call[0]).includes("/screenshot")).length;
}

async function mountStarted(agentId = "agent-1") {
  const hook = renderHook(() => useRemoteDesktop(agentId));
  // Let the version fetch resolve so startMonitoring is not version-gated.
  await act(async () => {
    await Promise.resolve();
  });
  await act(async () => {
    await hook.result.current.startMonitoring();
  });
  return hook;
}

beforeEach(() => {
  vi.clearAllMocks();
  wsMock.handler = null;
  hooksMock.cb = null;
  hooksMock.delay = 0;
  vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
    cb(0);
    return 1;
  });
  mockedGet.mockImplementation(async (path: string) => {
    if (path === "/agents/agent-1") return { agent: { version: "3.5.0" } };
    return { image: "AAAA", width: 100, height: 50 };
  });
  mockedPost.mockResolvedValue({ success: true });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useRemoteDesktop hardening", () => {
  it("posts the stream interval in seconds, not milliseconds", async () => {
    await mountStarted();

    const startCall = mockedPost.mock.calls.find((call) => String(call[0]).includes("/screen/start"));
    expect(startCall).toBeDefined();
    // Default resolution is medium (1000ms), which the server only accepts as
    // "1"-second units; the raw ms value used to be rejected with 400.
    expect(startCall?.[1]).toEqual({ interval: "1" });
  });

  it("skips the HTTP fallback poll while WS frames are live, and resumes when they stop", async () => {
    const nowSpy = vi.spyOn(Date, "now");
    nowSpy.mockReturnValue(1_000_000);
    const { result } = await mountStarted();
    expect(result.current.monitoring).toBe(true);
    // The forced first capture after start goes out over HTTP.
    expect(screenshotCalls()).toBe(1);

    act(() => {
      wsMock.handler?.({ type: "screenshot", agent_id: "agent-1", data: "AAAA" });
    });

    // Fresh WS frame: the poll tick must not fire another HTTP request.
    await act(async () => {
      hooksMock.cb?.();
      await Promise.resolve();
    });
    expect(screenshotCalls()).toBe(1);

    // Once the WS stream is stale (10s > max(5s, 2.5 polls)), polling resumes.
    nowSpy.mockReturnValue(1_010_000);
    await act(async () => {
      hooksMock.cb?.();
      await Promise.resolve();
    });
    expect(screenshotCalls()).toBe(2);
    nowSpy.mockRestore();
  });

  it("resets the WS-live shortcut when monitoring stops", async () => {
    const nowSpy = vi.spyOn(Date, "now");
    nowSpy.mockReturnValue(2_000_000);
    const { result } = await mountStarted();
    act(() => {
      wsMock.handler?.({ type: "screenshot", agent_id: "agent-1", data: "AAAA" });
    });

    await act(async () => {
      await result.current.stopMonitoring();
    });
    expect(result.current.monitoring).toBe(false);

    await act(async () => {
      await result.current.startMonitoring();
    });
    // Without the reset, the stale timestamp would suppress the forced first
    // capture of the new session.
    expect(screenshotCalls()).toBe(2);
    nowSpy.mockRestore();
  });
});
