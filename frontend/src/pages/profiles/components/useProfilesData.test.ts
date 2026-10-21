import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useProfilesData } from "./useProfilesData";

const { getMock, toastError, tMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  toastError: vi.fn(),
  tMock: (key: string) => key,
}));

vi.mock("@/lib/api", () => ({
  api: {
    get: (...args: unknown[]) => getMock(...args),
    postJson: vi.fn(),
    del: vi.fn(),
    postFormData: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: (...args: unknown[]) => toastError(...args), success: vi.fn(), warning: vi.fn() },
}));

vi.mock("@/lib/i18n", () => ({
  getLocale: () => "en",
  useI18n: () => ({ t: tMock }),
}));

describe("useProfilesData", () => {
  beforeEach(() => {
    getMock.mockReset();
    toastError.mockReset();
  });

  it("loads profiles and selects the first editable source", async () => {
    getMock.mockResolvedValue({
      profiles: [
        { name: "default", builtin: true, source: 'set sleeptime "15000";' },
        { name: "custom", builtin: false, source: 'set sleeptime "30000";' },
      ],
    });

    const { result } = renderHook(() => useProfilesData());
    await waitFor(() => expect(result.current.loadingProfiles).toBe(false));

    expect(result.current.profiles).toHaveLength(2);
    expect(result.current.selectedIdx).toBe(0);
    expect(result.current.source).toContain("15000");
    expect(result.current.savedSource).toBe(result.current.source);
    expect(result.current.dirty).toBe(false);
  });

  it("surfaces a profile-list read failure", async () => {
    getMock.mockRejectedValue(new Error("profiles down"));

    const { result } = renderHook(() => useProfilesData());
    await waitFor(() => expect(result.current.profilesError).toBe("profiles down"));

    expect(result.current.loadingProfiles).toBe(false);
    expect(toastError).toHaveBeenCalledWith("profiles down");
  });
});
