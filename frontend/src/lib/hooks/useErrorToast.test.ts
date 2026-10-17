import { act, renderHook } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const mocks = vi.hoisted(() => ({
  toast: vi.fn(),
  t: vi.fn((key: string) => `T:${key}`),
}));

vi.mock("sonner", () => ({ toast: { error: mocks.toast } }));
vi.mock("@/lib/i18n", () => ({ useI18n: () => ({ t: mocks.t }) }));

import { useErrorToast } from "./useErrorToast";

describe("useErrorToast", () => {
  beforeEach(() => {
    mocks.toast.mockClear();
    mocks.t.mockClear();
  });

  it("returns aborted errors without showing a toast", () => {
    const { result } = renderHook(() => useErrorToast());
    let returned: { kind: string } | undefined;

    act(() => {
      returned = result.current.toastError(new DOMException("go away", "AbortError"));
    });

    expect(mocks.toast).not.toHaveBeenCalled();
    expect(returned?.kind).toBe("aborted");
  });

  it("resolves mapped API errors through t", () => {
    const { result } = renderHook(() => useErrorToast());

    act(() => {
      result.current.toastError({ status: 403, message: "nope" });
    });

    expect(mocks.t).toHaveBeenCalledWith("common.forbidden", undefined);
    expect(mocks.toast).toHaveBeenCalledWith("T:common.forbidden");
  });

  it("falls back to the raw message for unmapped errors", () => {
    const { result } = renderHook(() => useErrorToast());

    act(() => {
      result.current.toastError(new Error("disk offline"));
    });

    expect(mocks.t).not.toHaveBeenCalled();
    expect(mocks.toast).toHaveBeenCalledWith("disk offline");
  });

  it("falls back to the fallback key when the message is empty", () => {
    const { result } = renderHook(() => useErrorToast());

    act(() => {
      result.current.toastError(new Error(""), "common.error");
    });

    expect(mocks.t).toHaveBeenCalledWith("common.error");
    expect(mocks.toast).toHaveBeenCalledWith("T:common.error");
  });
});
