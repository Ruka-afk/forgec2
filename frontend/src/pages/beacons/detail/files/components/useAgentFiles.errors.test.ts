import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { toast } from "sonner";
import { useAgentFiles } from "./useAgentFiles";
import { api } from "@/lib/api";
import { pushLocalFile } from "@/lib/agent-files/file-transfer";

vi.mock("@/lib/agent-files/file-transfer", () => ({
  pushLocalFile: vi.fn(),
  pullRemoteFile: vi.fn(),
}));

// Stable t identity mirrors the real useI18n (see useAgentFiles.upload.test.ts):
// a fresh arrow per render would invalidate every error-handling useCallback.
const { stableT } = vi.hoisted(() => ({ stableT: (k: string) => k }));
vi.mock("@/lib/i18n", () => ({
  getLocale: () => "en",
  useI18n: () => ({ t: stableT, lang: "en", setLang: vi.fn() }),
}));

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn(), get: vi.fn(), postJson: vi.fn() },
  pollTask: vi.fn(),
  getRateLimitRetryAfter: vi.fn(() => 7),
}));

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  }),
}));

const mockedPost = vi.mocked(api.post);
const mockedGet = vi.mocked(api.get);
const mockedPush = vi.mocked(pushLocalFile);
const mockedToastError = vi.mocked(toast.error);

const lsResult = (names: string[]) => ({
  files: names.map((name) => ({ name, is_dir: false, size: 10 })),
});

/** ApiError-shaped rejection: status + message is what `normalizeError` keys on. */
const apiError = (status: number, message: string) =>
  Object.assign(new Error(message), { status });

async function flush(times = 4) {
  for (let i = 0; i < times; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

/** Mount with OS detected and the first listing served, matching a ready browser. */
async function mountLoaded() {
  const { result } = renderHook(() => useAgentFiles("agent-1"));
  await flush();
  return result;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedGet.mockResolvedValue({ agent: { os: "Windows" } });
  mockedPost.mockResolvedValue(lsResult(["a.txt"]));
});

describe("useAgentFiles error surfaces", () => {
  it("maps a 5xx listing failure for both the toast and the retry card", async () => {
    const result = await mountLoaded();
    expect(result.current.entries).toHaveLength(1);

    mockedPost.mockRejectedValueOnce(apiError(500, "internal exploded"));
    await act(async () => {
      await result.current.loadDirectory("D:\\");
    });
    await flush(1);

    expect(mockedToastError).toHaveBeenCalledTimes(1);
    expect(mockedToastError).toHaveBeenCalledWith("common.server_error");
    expect(result.current.listError).toBe("common.server_error");
    // The last good listing stays visible behind the retry card.
    expect(result.current.entries).toHaveLength(1);
  });

  it("passes backend 4xx text through to both the toast and the card", async () => {
    const result = await mountLoaded();

    mockedPost.mockRejectedValueOnce(apiError(400, "D:\\ does not exist"));
    await act(async () => {
      await result.current.loadDirectory("D:\\");
    });
    await flush(1);

    expect(mockedToastError).toHaveBeenLastCalledWith("D:\\ does not exist");
    expect(result.current.listError).toBe("D:\\ does not exist");
  });

  it("maps 429 to the shared rate-limit message", async () => {
    const result = await mountLoaded();

    mockedPost.mockRejectedValueOnce(apiError(429, "slow down"));
    await act(async () => {
      await result.current.loadDirectory("D:\\");
    });
    await flush(1);

    expect(mockedToastError).toHaveBeenLastCalledWith("common.rate_limited");
  });

  it("falls back to the locale copy when a failure carries no message", async () => {
    const result = await mountLoaded();

    mockedPost.mockRejectedValueOnce(new Error(""));
    await act(async () => {
      await result.current.deleteFile("a.txt");
    });
    await flush(1);

    expect(mockedToastError).toHaveBeenLastCalledWith("agents.files_delete_failed");
  });

  it("shows a local Error message verbatim, without the old String(err) prefix", async () => {
    const result = await mountLoaded();

    mockedPost.mockRejectedValueOnce(new Error("queue ack on the next beacon"));
    await act(async () => {
      await result.current.readFile("a.txt");
    });
    await flush(1);

    expect(mockedToastError).toHaveBeenLastCalledWith("queue ack on the next beacon");
  });

  it("stays silent when the upload pump rejects with an AbortError", async () => {
    mockedPush.mockRejectedValueOnce(new DOMException("aborted", "AbortError"));
    const result = await mountLoaded();

    act(() => {
      result.current.uploadFile(new File(["a"], "a.txt"));
    });
    await flush(3);

    // Aborts are operator-initiated, not failures: no error toast.
    expect(mockedToastError).not.toHaveBeenCalled();
  });

  it("prefixes batch-delete failures with the file and maps the status", async () => {
    const result = await mountLoaded();
    mockedPost.mockImplementation(async (url: string, data?: Record<string, string>) => {
      if (String(url).endsWith("/files/delete")) {
        if (String(data?.path ?? "").endsWith("a.txt")) throw apiError(500, "boom");
        return { success: true };
      }
      return lsResult(["a.txt", "b.txt"]);
    });

    await act(async () => {
      await result.current.deleteFiles(["a.txt", "b.txt"]);
    });
    await flush(2);

    const calls = mockedToastError.mock.calls.map((c) => String(c[0]));
    expect(calls).toContain("a.txt: common.server_error");
    expect(calls).toContain("agents.files_batch_deleted");
  });

  it("maps a TypeError from OS detection to the network diagnosis", async () => {
    mockedGet.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    const { result } = renderHook(() => useAgentFiles("agent-1"));
    await flush();

    expect(result.current.osError).toBe("common.network_error");
  });
});
