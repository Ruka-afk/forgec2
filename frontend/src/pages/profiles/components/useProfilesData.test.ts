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
    post: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: (...args: unknown[]) => toastError(...args), success: vi.fn() },
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: tMock }),
}));

describe("useProfilesData", () => {
  beforeEach(() => {
    getMock.mockReset();
    toastError.mockReset();
  });

  it("reads /settings once, not once per consumer", async () => {
    // The hook used to fire two identical GET /settings calls: one for the
    // read-only card and one (loadMalleableSettings) to seed a local edit form
    // that was a duplicate of the Settings editor. The form is gone, so the
    // card must be the only consumer.
    getMock.mockImplementation((path: string) => {
      if (path === "/settings") {
        return Promise.resolve({
          malleable_enabled: true,
          malleable_status: 201,
          malleable_ct: "text/html",
          malleable_headers: { Server: "cloudflare" },
          malleable_prepend: "<!--x",
          malleable_append: "x-->",
        });
      }
      return Promise.resolve({ profiles: [] });
    });

    const { result } = renderHook(() => useProfilesData());
    await waitFor(() => expect(result.current.loadingActiveConfig).toBe(false));

    const settingsCalls = getMock.mock.calls.filter(([p]) => p === "/settings");
    expect(settingsCalls).toHaveLength(1);
  });

  it("surfaces an active-config read failure instead of showing it as disabled", async () => {
    // A failing /settings call must be reported rather than rendered as
    // "disabled", which would read as "the operator turned camouflage off".
    getMock.mockImplementation((path: string) => {
      if (path === "/settings") return Promise.reject(new Error("settings down"));
      return Promise.resolve({ profiles: [] });
    });

    const { result } = renderHook(() => useProfilesData());
    await waitFor(() => expect(result.current.activeConfigError).toBe("settings down"));

    expect(result.current.loadingActiveConfig).toBe(false);
  });

  it("reads the active card's enabled state from /settings, not a key that does not exist", async () => {
    // Regression lock: the card used /integrations/malleable, which returns a
    // bare "enabled" and no "malleable_enabled" at all, so the toggle always
    // read undefined and always rendered "disabled" regardless of the real
    // setting. The card also used to fabricate "0s / 0%" for interval/jitter,
    // which no endpoint returns.
    getMock.mockImplementation((path: string) => {
      if (path === "/settings") {
        return Promise.resolve({
          malleable_enabled: true,
          malleable_status: 418,
          malleable_ct: "text/plain",
          malleable_headers: { Server: "nginx" },
          malleable_prepend: "<p>",
          malleable_append: "</p>",
        });
      }
      return Promise.resolve({ profiles: [] });
    });

    const { result } = renderHook(() => useProfilesData());
    await waitFor(() => expect(result.current.loadingActiveConfig).toBe(false));

    expect(result.current.activeConfig.malleable_enabled).toBe(true);
    expect(result.current.activeConfig.status_code).toBe(418);
    expect(result.current.activeConfig.content_type).toBe("text/plain");
    expect(result.current.activeConfig.headers).toEqual({ Server: "nginx" });
    expect(result.current.activeConfigError).toBeNull();
  });
});
