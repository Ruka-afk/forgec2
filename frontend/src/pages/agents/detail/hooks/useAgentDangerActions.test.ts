import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";

const mocks = vi.hoisted(() => ({
  t: vi.fn((key: string) => `T:${key}`),
}));

vi.mock("@/lib/i18n", () => ({ useI18n: () => ({ t: mocks.t }) }));

import { useAgentDangerActions } from "./useAgentDangerActions";
import { api } from "@/lib/api";
import { toast } from "sonner";

function setup(agentId = "a1") {
  return renderHook(() => useAgentDangerActions(agentId, () => {}));
}

describe("useAgentDangerActions", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    mocks.t.mockClear();
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
    expect(toast.success).toHaveBeenCalledWith("T:agents.detail_kill_date_set");
  });

  it("resolves false instead of rejecting when the call fails", async () => {
    vi.spyOn(api, "postJson").mockRejectedValue(new Error("network"));
    const { result } = setup();

    let ok: boolean | undefined;
    await act(async () => { ok = await result.current.setKillDate("2026-01-01"); });

    expect(ok).toBe(false);
    // Unmapped errors keep their raw message (normalizeError convention).
    expect(toast.error).toHaveBeenCalledWith("network");
  });

  it("maps API error statuses through useErrorToast", async () => {
    vi.spyOn(api, "postJson").mockRejectedValue({ status: 403, message: "nope" });
    const { result } = setup();

    await act(async () => { await result.current.killAgent(); });

    expect(mocks.t).toHaveBeenCalledWith("common.forbidden", undefined);
    expect(toast.error).toHaveBeenCalledWith("T:common.forbidden");
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

  it("tracks busy across the in-flight window", async () => {
    let release!: () => void;
    vi.spyOn(api, "postJson").mockImplementation(
      () => new Promise((resolve) => { release = () => resolve({} as never); }),
    );
    const { result } = setup();

    expect(result.current.busy).toBe(false);
    await act(async () => { void result.current.killAgent(); });
    expect(result.current.busy).toBe(true);

    await act(async () => { release(); });
    expect(result.current.busy).toBe(false);
  });

  it("reports false when there is no agent to act on", async () => {
    const post = vi.spyOn(api, "postJson");
    const { result } = renderHook(() => useAgentDangerActions("", () => {}));

    let ok: boolean | undefined;
    await act(async () => { ok = await result.current.killAgent(); });

    expect(ok).toBe(false);
    expect(post).not.toHaveBeenCalled();
  });
});
