import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useAgentQuickShell } from "./useAgentQuickShell";
import { api } from "@/lib/api";

function setup(agentId = "a1") {
  return renderHook(
    ({ id }: { id: string }) => useAgentQuickShell(id, "Windows", "sent", "send failed"),
    { initialProps: { id: agentId } },
  );
}

describe("useAgentQuickShell", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
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
});
