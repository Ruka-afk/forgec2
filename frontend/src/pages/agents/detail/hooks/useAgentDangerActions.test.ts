import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useAgentDangerActions } from "./useAgentDangerActions";
import { api } from "@/lib/api";
import { toast } from "sonner";

const MESSAGES = {
  killSuccess: "killed",
  killFailed: "kill failed",
  uninstallSuccess: "uninstalled",
  uninstallFailed: "uninstall failed",
  killDateSuccess: "date set",
  killDateFailed: "date failed",
  clearKillDateSuccess: "date cleared",
  clearKillDateFailed: "clear failed",
  migrateSuccess: "migrated",
  migrateFailed: "migrate failed",
};

function setup() {
  return renderHook(() => useAgentDangerActions("a1", () => {}, MESSAGES));
}

describe("useAgentDangerActions", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(toast, "success").mockImplementation(() => "");
    vi.spyOn(toast, "error").mockImplementation(() => "");
  });

  // The reason these actions return a boolean: they swallow the error and
  // toast it, so a caller written as `await run(); closeDialog()` closed the
  // dialog on failure too, discarding what the operator had typed into it.
  it("reports success so the caller can keep its dialog open on failure", async () => {
    vi.spyOn(api, "postJson").mockResolvedValue({} as never);
    const { result } = setup();

    let ok: boolean | undefined;
    await act(async () => { ok = await result.current.setKillDate("2026-01-01"); });
    expect(ok).toBe(true);
    expect(toast.success).toHaveBeenCalledWith(MESSAGES.killDateSuccess);
  });

  it("resolves false instead of rejecting when the call fails", async () => {
    vi.spyOn(api, "postJson").mockRejectedValue(new Error("network"));
    const { result } = setup();

    let ok: boolean | undefined;
    await act(async () => { ok = await result.current.setKillDate("2026-01-01"); });

    expect(ok).toBe(false);
    expect(toast.error).toHaveBeenCalledWith(MESSAGES.killDateFailed);
  });

  it("reports failure for the other four actions too", async () => {
    vi.spyOn(api, "postJson").mockRejectedValue(new Error("network"));
    vi.spyOn(api, "post").mockRejectedValue(new Error("network"));
    vi.spyOn(api, "del").mockRejectedValue(new Error("network"));
    const { result } = setup();

    const out: boolean[] = [];
    await act(async () => {
      out.push(await result.current.killAgent());
      out.push(await result.current.uninstallAgent());
      out.push(await result.current.clearKillDate());
      out.push(await result.current.migrateAgent("/x"));
    });
    expect(out).toEqual([false, false, false, false]);
  });

  it("always clears busy, so the disabled button cannot get stuck", async () => {
    vi.spyOn(api, "postJson").mockRejectedValue(new Error("network"));
    const { result } = setup();

    expect(result.current.busy).toBeNull();
    await act(async () => { await result.current.killAgent(); });
    expect(result.current.busy).toBeNull();
  });

  it("reports false when there is no agent to act on", async () => {
    const post = vi.spyOn(api, "postJson");
    const { result } = renderHook(() => useAgentDangerActions("", () => {}, MESSAGES));

    let ok: boolean | undefined;
    await act(async () => { ok = await result.current.killAgent(); });

    expect(ok).toBe(false);
    expect(post).not.toHaveBeenCalled();
  });
});
