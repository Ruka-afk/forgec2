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

  it("keeps the server's malleable headers so the profiles save cannot wipe them", async () => {
    getMock.mockImplementation((path: string) => {
      if (path === "/settings") {
        return Promise.resolve({
          malleable_enabled: true,
          malleable_status: 201,
          malleable_ct: "text/html",
          malleable_headers: "Server: cloudflare",
          malleable_prepend: "<!--x",
          malleable_append: "x-->",
        });
      }
      return Promise.resolve({ profiles: [] });
    });

    const { result } = renderHook(() => useProfilesData());
    await waitFor(() => expect(result.current.malleableLoaded).toBe(true));

    expect(result.current.malleableForm.headers_text).toBe("Server: cloudflare");
    expect(result.current.malleableForm.status_code).toBe(201);
    expect(result.current.malleableForm.content_type).toBe("text/html");
  });

  it("blocks the malleable save when the settings read failed", async () => {
    getMock.mockImplementation((path: string) => {
      if (path === "/settings") return Promise.reject(new Error("settings down"));
      return Promise.resolve({ profiles: [] });
    });

    const { result } = renderHook(() => useProfilesData());
    await waitFor(() => expect(result.current.malleableError).toBe("settings down"));

    expect(result.current.malleableLoaded).toBe(false);
  });

  it("surfaces an active-config read failure instead of showing it as disabled", async () => {
    // The card and the editable form now share one read, so a single failing
    // /settings call must be reported rather than rendered as "disabled".
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
