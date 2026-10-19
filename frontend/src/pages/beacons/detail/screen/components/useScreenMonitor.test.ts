import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useScreenMonitor } from "./useScreenMonitor";
import { api } from "@/lib/api";

const toastMock = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
const wsMock = vi.hoisted(() => ({ handler: null as null | ((msg: Record<string, unknown>) => void) }));

vi.mock("@/lib/hooks/useVisibleInterval", () => ({
  useVisibleInterval: () => {},
}));

vi.mock("@/lib/wsContext", () => ({
  useWS: () => ({
    subscribe: (handler: (msg: Record<string, unknown>) => void) => {
      wsMock.handler = handler;
      return () => {
        wsMock.handler = null;
      };
    },
  }),
}));

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/screenImage", () => ({
  dataUrlToBlob: vi.fn(),
  dataUrlToBlobUrl: vi.fn(() => "blob:test"),
  revokeBlobUrl: vi.fn(),
}));

vi.mock("@/lib/i18n", () => {
  // `t` must be referentially stable: mutation callbacks carry it.
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: { success: toastMock.success, error: toastMock.error },
}));

const t = (key: string) => key;

function mount(agentId = "agent-1") {
  return renderHook(() =>
    useScreenMonitor(agentId, { interval: 5, quality: "medium", autoRefresh: false, t }),
  );
}

async function settle() {
  await act(async () => {
    await Promise.resolve();
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  wsMock.handler = null;
  vi.mocked(api.get).mockResolvedValue({});
  vi.mocked(api.post).mockResolvedValue({});
});

describe("useScreenMonitor error routing", () => {
  it("normalizes a rejected monitor start", async () => {
    vi.mocked(api.post).mockRejectedValue({ status: 400, message: "interval must be between 1 and 60 seconds" });
    const { result } = mount();
    await settle();

    await act(async () => {
      await result.current.startMonitoring();
    });

    expect(toastMock.error).toHaveBeenCalledWith("interval must be between 1 and 60 seconds");
    expect(result.current.status).toBe("error");
  });

  it("normalizes a rejected monitor stop through the mapped status", async () => {
    vi.mocked(api.post).mockRejectedValue({ status: 500, message: "boom" });
    const { result } = mount();
    await settle();

    await act(async () => {
      await result.current.stopMonitoring();
    });

    expect(toastMock.error).toHaveBeenCalledWith("common.server_error");
  });

  it("normalizes a rejected on-demand capture", async () => {
    vi.mocked(api.post).mockRejectedValue({ status: 400, message: "no window matches" });
    const { result } = mount();
    await settle();

    await act(async () => {
      await result.current.requestFreshCapture("window");
    });

    expect(toastMock.error).toHaveBeenCalledWith("no window matches");
  });

  it("falls back to the trigger key when the rejection carries no message", async () => {
    vi.mocked(api.post).mockRejectedValue(new Error(""));
    const { result } = mount();
    await settle();

    await act(async () => {
      await result.current.startTitleTrigger("notepad");
    });

    expect(toastMock.error).toHaveBeenCalledWith("agents.trigger_failed");
  });
});
