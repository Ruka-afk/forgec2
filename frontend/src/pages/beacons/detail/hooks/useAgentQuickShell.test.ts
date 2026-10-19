import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import { useAgentQuickShell } from "./useAgentQuickShell";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() },
}));

vi.mock("@/lib/i18n", () => {
  // `t` must be referentially stable: the send/load callbacks carry it.
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

function setup(agentId = "a1") {
  return renderHook(
    ({ id }: { id: string }) => useAgentQuickShell(id, "Windows", "sent", "send failed"),
    { initialProps: { id: agentId } },
  );
}

describe("useAgentQuickShell", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.mocked(toast.warning).mockClear();
    window.localStorage.clear();
  });

  it("records the command with its output, then clears the input", async () => {
    vi.spyOn(api, "postJson").mockResolvedValue({ output: "hi" } as never);
    const { result } = setup();

    act(() => { result.current.setCommand("whoami"); });
    await act(async () => { await result.current.sendCommand(); });

    expect(result.current.history).toHaveLength(1);
    expect(result.current.history[0]).toMatchObject({ command: "whoami", result: "hi" });
    expect(result.current.command).toBe("");
    expect(result.current.sending).toBe(false);
  });

  it("drops a second send while one is in flight (Enter-key guard)", async () => {
    let release!: () => void;
    vi.spyOn(api, "postJson").mockImplementation(
      () => new Promise((resolve) => { release = () => resolve({ output: "hi" } as never); }),
    );
    const { result } = setup();

    act(() => { result.current.setCommand("whoami"); });
    await act(async () => { void result.current.sendCommand(); });
    expect(result.current.sending).toBe(true);

    await act(async () => { void result.current.sendCommand(); });
    expect(api.postJson).toHaveBeenCalledTimes(1);

    await act(async () => { release(); });
    expect(result.current.sending).toBe(false);
    expect(result.current.history).toHaveLength(1);
  });

  it("abandons an in-flight command when the agent id changes", async () => {
    let release!: () => void;
    vi.spyOn(api, "postJson").mockImplementation(
      () => new Promise((resolve) => { release = () => resolve({ output: "hi" } as never); }),
    );
    const { result, rerender } = setup("a1");

    act(() => { result.current.setCommand("whoami"); });
    await act(async () => { void result.current.sendCommand(); });
    expect(result.current.sending).toBe(true);

    rerender({ id: "a2" });
    expect(result.current.sending).toBe(false);

    await act(async () => { release(); });
    // The previous agent's result must not land in the new agent's history.
    expect(result.current.history).toHaveLength(0);

    // ...and its guard must not keep blocking sends to the new agent.
    act(() => { result.current.setCommand("hostname"); });
    await act(async () => { void result.current.sendCommand(); });
    expect(api.postJson).toHaveBeenCalledTimes(2);
    await act(async () => { release(); });
    expect(result.current.history).toHaveLength(1);
  });

  it("treats a 409 as a warning outcome and records the reason in history", async () => {
    vi.spyOn(api, "postJson").mockRejectedValueOnce(new ApiError("approval required", 409));
    const { result } = setup();

    act(() => { result.current.setCommand("whoami"); });
    await act(async () => { void result.current.sendCommand(); });

    await waitFor(() => expect(vi.mocked(toast.warning)).toHaveBeenCalledWith("approval required"));
    expect(result.current.history[0]).toMatchObject({ command: "whoami", result: "approval required" });
    expect(result.current.sending).toBe(false);
    expect(result.current.command).toBe("");
  });

  it("falls back to the send-failure copy for a message-less 409 instead of a blank row", async () => {
    vi.spyOn(api, "postJson").mockRejectedValueOnce(new ApiError("", 409));
    const { result } = setup();

    act(() => { result.current.setCommand("hostname"); });
    await act(async () => { void result.current.sendCommand(); });

    await waitFor(() => expect(vi.mocked(toast.warning)).toHaveBeenCalledWith("agents.detail_command_send_failed"));
    expect(result.current.history[0]?.result).toBe("agents.detail_command_send_failed");
  });

  it("records a non-conflict failure with the pre-translated message and no warning", async () => {
    vi.spyOn(api, "postJson").mockRejectedValueOnce({ status: 500, message: "command backend exploded" });
    const { result } = setup();

    act(() => { result.current.setCommand("whoami"); });
    await act(async () => { void result.current.sendCommand(); });

    expect(result.current.history[0]).toMatchObject({ command: "whoami", result: "send failed" });
    expect(vi.mocked(toast.warning)).not.toHaveBeenCalled();
    expect(result.current.sending).toBe(false);
  });
});
