import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";

const mocks = vi.hoisted(() => ({
  t: vi.fn((key: string) => `T:${key}`),
}));

vi.mock("@/lib/i18n", () => ({ useI18n: () => ({ t: mocks.t }) }));

import { useAgentNotes } from "./useAgentNotes";
import { api } from "@/lib/api";
import { toast } from "sonner";

describe("useAgentNotes", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(toast, "success").mockImplementation(() => "");
    vi.spyOn(toast, "error").mockImplementation(() => "");
  });

  it("closes the editor and reloads the detail on a successful save", async () => {
    vi.spyOn(api, "post").mockResolvedValue({} as never);
    const reload = vi.fn();
    const { result } = renderHook(() => useAgentNotes("a1", reload));

    act(() => { result.current.startEditing("tag", "note"); });
    expect(result.current.editing).toBe(true);

    await act(async () => { await result.current.save(); });

    expect(result.current.editing).toBe(false);
    expect(reload).toHaveBeenCalledTimes(1);
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("keeps the editor open and maps the failure through useErrorToast", async () => {
    vi.spyOn(api, "post").mockRejectedValue({ status: 403, message: "nope" });
    const reload = vi.fn();
    const { result } = renderHook(() => useAgentNotes("a1", reload));

    act(() => { result.current.startEditing("tag", "note"); });
    await act(async () => { await result.current.save(); });

    expect(result.current.editing).toBe(true);
    expect(reload).not.toHaveBeenCalled();
    expect(toast.error).toHaveBeenCalledWith("T:common.forbidden");
  });

  it("abandons an in-flight save when the agent id changes", async () => {
    let release!: () => void;
    vi.spyOn(api, "post").mockImplementation(
      () => new Promise((resolve) => { release = () => resolve({} as never); }),
    );
    const reload = vi.fn();
    const { result, rerender } = renderHook(
      ({ id }: { id: string }) => useAgentNotes(id, reload),
      { initialProps: { id: "a1" } },
    );

    act(() => { result.current.startEditing("tag", "note"); });
    await act(async () => { void result.current.save(); });
    expect(result.current.saving).toBe(true);

    rerender({ id: "a2" });
    expect(result.current.saving).toBe(false);
    expect(result.current.editing).toBe(false);

    await act(async () => { release(); });
    // The previous agent's completion must not reload or toast against the new one.
    expect(reload).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });
});
