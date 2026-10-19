import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useAgentMenuAction } from "./useAgentMenuAction";
import type { Beacon } from "./types";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn(), postJson: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }),
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);

const beacon = { id: "a1", hostname: "host1", status: "online" } as Beacon;
const point = { x: 0, y: 0, beacon };

function renderMenu() {
  const setMenuPoint = vi.fn();
  const deps = {
    t: (key: string) => key,
    navigate: vi.fn(),
    setSelectedAgentId: vi.fn(),
    setMenuPoint,
    openQuickSleep: vi.fn(),
    openNotesEdit: vi.fn(),
    setConfirm: vi.fn(),
  };
  const view = renderHook(() => useAgentMenuAction(deps));
  return { ...view, setMenuPoint };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("useAgentMenuAction", () => {
  it("queues a screenshot task and reports it as queued", async () => {
    const { result } = renderMenu();
    vi.mocked(api.post).mockResolvedValue({ task_id: 3 });

    act(() => {
      result.current.handleMenuAction("screenshot", point);
    });

    await waitFor(() => expect(successToast).toHaveBeenCalledWith("agents.detail_action_queued"));
    expect(api.post).toHaveBeenCalledWith(paths.agents.screenshotTask("a1"));
  });

  it("surfaces the backend reason when the screenshot task is rejected", async () => {
    const { result } = renderMenu();
    vi.mocked(api.post).mockRejectedValue({ status: 400, message: "agent offline" });

    act(() => {
      result.current.handleMenuAction("screenshot", point);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("agent offline"));
  });

  it("normalizes a failed beacon-now queue into its mapped copy", async () => {
    const { result } = renderMenu();
    vi.mocked(api.post).mockRejectedValue({ status: 500, message: "boom" });

    act(() => {
      result.current.handleMenuAction("beacon_now", point);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
  });

  it("closes the menu after dispatching and leaves clipboard failures direct", async () => {
    const { result, setMenuPoint } = renderMenu();
    const writeText = vi.fn().mockRejectedValue(new Error("denied"));
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });

    act(() => {
      result.current.handleMenuAction("copy_id", point);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("agents.detail_copy_failed"));
    expect(setMenuPoint).toHaveBeenCalledWith(null);
  });
});
