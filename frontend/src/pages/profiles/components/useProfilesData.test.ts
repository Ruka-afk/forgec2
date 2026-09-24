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
      if (path === "/integrations/malleable") {
        return Promise.resolve({ malleable_enabled: true, status_code: 201 });
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
      if (path === "/integrations/malleable") return Promise.resolve({});
      return Promise.resolve({ profiles: [] });
    });

    const { result } = renderHook(() => useProfilesData());
    await waitFor(() => expect(result.current.malleableError).toBe("settings down"));

    expect(result.current.malleableLoaded).toBe(false);
  });

  it("surfaces an active-config read failure instead of showing it as disabled", async () => {
    getMock.mockImplementation((path: string) => {
      if (path === "/settings") return Promise.resolve({});
      if (path === "/integrations/malleable") return Promise.reject(new Error("malleable down"));
      return Promise.resolve({ profiles: [] });
    });

    const { result } = renderHook(() => useProfilesData());
    await waitFor(() => expect(result.current.activeConfigError).toBe("malleable down"));

    expect(result.current.loadingActiveConfig).toBe(false);
  });
});
