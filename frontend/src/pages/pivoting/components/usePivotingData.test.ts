import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { usePivotingData } from "./usePivotingData";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.get).mockResolvedValue({});
});

describe("usePivotingData failure reporting", () => {
  it("surfaces the backend reason when an RPort start is rejected", async () => {
    const { result } = renderHook(() => usePivotingData());

    vi.mocked(api.post).mockRejectedValue({ status: 400, message: "port in use" });
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.startRPort({
        rportAgent: "a1",
        remoteHost: "10.0.0.9",
        remotePort: 445,
        localPort: 8022,
        protocol: "tcp",
      });
    });

    expect(ok).toBe(false);
    expect(errorToast).toHaveBeenCalledWith("port in use");
    expect(api.post).toHaveBeenCalledWith(paths.agents.rportfwdStart("a1"), {
      lport: "8022",
      target: "10.0.0.9:445",
    });
  });

  it("localizes a server failure for a rejected relay start", async () => {
    const { result } = renderHook(() => usePivotingData());

    vi.mocked(api.post).mockRejectedValue({ status: 500, message: "boom" });
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.startRelay("a1", 1080, "127.0.0.1", "socks");
    });

    expect(ok).toBe(false);
    expect(errorToast).toHaveBeenCalledWith("common.server_error");
    expect(successToast).not.toHaveBeenCalled();
  });

  it("falls back to the locale copy when a stop rejection carries no message", async () => {
    const { result } = renderHook(() => usePivotingData());

    vi.mocked(api.post).mockRejectedValue(new Error(""));
    await act(async () => {
      await result.current.stopRPort("a1", 8022);
    });

    expect(errorToast).toHaveBeenCalledWith("pivoting.toast.rport_stop_failed");
  });

  it("reports a started relay after refreshing the list", async () => {
    const { result } = renderHook(() => usePivotingData());

    vi.mocked(api.post).mockResolvedValue({});
    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.startRelay("a1", 1080, "127.0.0.1", "socks");
    });

    expect(ok).toBe(true);
    expect(successToast).toHaveBeenCalledWith("pivoting.toast.socks_relay_started");
  });
});
