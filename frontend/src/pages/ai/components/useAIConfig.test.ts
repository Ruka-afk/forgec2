import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAIConfig } from "./useAIConfig";

const { getMock, postJsonMock, tMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postJsonMock: vi.fn(),
  tMock: (key: string) => key,
}));

vi.mock("@/lib/api", () => ({
  api: {
    get: (...args: unknown[]) => getMock(...args),
    postJson: (...args: unknown[]) => postJsonMock(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: tMock }),
}));

describe("useAIConfig", () => {
  beforeEach(() => {
    getMock.mockReset();
    postJsonMock.mockReset();
  });

  it("enables first-time setup immediately and clears the API key field", async () => {
    getMock
      .mockResolvedValueOnce({ AIConfig: { enabled: false, provider: "deepseek", model: "deepseek-chat", has_api_key: false } })
      .mockResolvedValueOnce({ AIConfig: { enabled: true, provider: "deepseek", model: "deepseek-chat", has_api_key: true, allow_execute: true } });
    postJsonMock.mockResolvedValueOnce({ success: true });

    const { result } = renderHook(() => useAIConfig());
    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(1));

    act(() => {
      result.current.setApiKey("secret-key");
      result.current.setEnabled(true);
      result.current.setAllowExecute(true);
      result.current.setShowSettings(true);
    });
    await act(async () => {
      await result.current.handleSaveConfig();
    });

    expect(postJsonMock).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({
      api_key: "secret-key",
      enabled: true,
      allow_execute: true,
    }));
    expect(result.current.apiKey).toBe("");
    expect(result.current.hasApiKey).toBe(true);
    expect(result.current.enabled).toBe(true);
    expect(result.current.allowExecute).toBe(true);
    expect(result.current.showSettings).toBe(false);
    expect(getMock).toHaveBeenCalledTimes(2);
  });

  it("never reports a failed config read as 'AI disabled with no key'", async () => {
    getMock.mockRejectedValueOnce(new Error("ai endpoint down"));

    const { result } = renderHook(() => useAIConfig());
    await waitFor(() => expect(getMock).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(result.current.configError).toBe("ai endpoint down"));

    // The defaults stay put, but the caller is told they are not real.
    expect(result.current.configLoaded).toBe(false);
    expect(result.current.configLoading).toBe(false);
  });

  it("refuses to save while the configuration is still unread", async () => {
    getMock.mockRejectedValueOnce(new Error("ai endpoint down"));

    const { result } = renderHook(() => useAIConfig());
    await waitFor(() => expect(result.current.configError).toBe("ai endpoint down"));

    await act(async () => {
      await result.current.handleSaveConfig();
    });

    expect(postJsonMock).not.toHaveBeenCalled();
  });

  it("allows the save once a real configuration has loaded", async () => {
    getMock.mockResolvedValue({ AIConfig: { enabled: true, provider: "deepseek", model: "deepseek-chat", has_api_key: true } });
    postJsonMock.mockResolvedValueOnce({ success: true });

    const { result } = renderHook(() => useAIConfig());
    await waitFor(() => expect(result.current.configLoaded).toBe(true));

    await act(async () => {
      await result.current.handleSaveConfig();
    });

    expect(postJsonMock).toHaveBeenCalledTimes(1);
  });
});
