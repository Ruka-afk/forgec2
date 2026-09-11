import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useAgentFiles } from "./useAgentFiles";
import { api, pollTask } from "@/lib/api";

vi.mock("@/lib/agent-files/file-transfer", () => ({
  pushLocalFile: vi.fn(),
  pullRemoteFile: vi.fn(),
}));

const { stableT } = vi.hoisted(() => ({ stableT: (k: string) => k }));
vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: stableT, lang: "en", setLang: vi.fn() }),
}));

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn(), get: vi.fn(), postJson: vi.fn() },
  pollTask: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

const mockedPost = vi.mocked(api.post);
const mockedGet = vi.mocked(api.get);
const mockedPoll = vi.mocked(pollTask);

const lsResult = (names: string[]) => ({
  files: names.map((name) => ({ name, is_dir: false, size: 10 })),
});

async function flush(times = 4) {
  for (let i = 0; i < times; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

function lsCalls() {
  return mockedPost.mock.calls.filter(([url]) => String(url).endsWith("/files/ls"));
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedGet.mockResolvedValue({});
  mockedPost.mockResolvedValue(lsResult(["a.txt"]));
});

describe("useAgentFiles listing cache", () => {
  it("serves repeat visits from cache without re-posting ls", async () => {
    const { result } = renderHook(() => useAgentFiles("agent-1"));
    await flush();
    expect(lsCalls()).toHaveLength(1);
    expect(result.current.entries).toHaveLength(1);

    await act(async () => {
      await result.current.loadDirectory("C:\\");
    });
    await flush(1);
    expect(lsCalls()).toHaveLength(1);
    expect(result.current.entries).toHaveLength(1);
  });

  it("refresh bypasses the cache and other paths still fetch", async () => {
    const { result } = renderHook(() => useAgentFiles("agent-1"));
    await flush();
    expect(lsCalls()).toHaveLength(1);

    await act(async () => {
      await result.current.loadDirectory("C:\\", { refresh: true });
    });
    await flush(1);
    expect(lsCalls()).toHaveLength(2);

    await act(async () => {
      await result.current.loadDirectory("D:\\");
    });
    await flush(1);
    expect(lsCalls()).toHaveLength(3);
  });

  it("mutations invalidate the listing", async () => {
    mockedPoll.mockResolvedValue({ status: "completed", result: "created directory x" } as never);
    const { result } = renderHook(() => useAgentFiles("agent-1"));
    await flush();
    expect(lsCalls()).toHaveLength(1);

    await act(async () => {
      await result.current.mkdir("newdir");
    });
    await flush(2);
    // mkdir POST answers with an ls-shaped object (no task_id → no poll);
    // the refresh reload still re-posts ls.
    expect(lsCalls().length).toBeGreaterThanOrEqual(2);
  });

  it("chmodFile posts path+mode and polls the task", async () => {
    mockedPost.mockImplementation(async (url: string) => {
      if (String(url).endsWith("/files/chmod")) return { task_id: 7, success: true };
      return lsResult(["a.txt"]);
    });
    mockedPoll.mockResolvedValue({ status: "completed", result: "set mode 644" } as never);
    const { result } = renderHook(() => useAgentFiles("agent-1"));
    await flush();

    await act(async () => {
      await result.current.chmodFile("a.txt", "644");
    });
    await flush(2);

    const chmod = mockedPost.mock.calls.find(([url]) => String(url).endsWith("/files/chmod"));
    expect(chmod).toBeDefined();
    expect(chmod?.[1]).toEqual({ path: "C:\\a.txt", mode: "644" });
    expect(mockedPoll).toHaveBeenCalledWith("agent-1", 7, expect.objectContaining({ timeoutMs: 90_000 }));
  });

  it("readFile refuses oversized previews without posting", async () => {
    const { result } = renderHook(() => useAgentFiles("agent-1"));
    mockedPost.mockResolvedValue({
      files: [{ name: "big.bin", is_dir: false, size: 8 * 1024 * 1024 }],
    });
    await act(async () => {
      await result.current.loadDirectory("C:\\", { refresh: true });
    });
    await flush(1);
    const before = mockedPost.mock.calls.length;

    await act(async () => {
      await result.current.readFile("big.bin");
    });
    await flush(1);
    expect(mockedPost.mock.calls.length).toBe(before);
    expect(result.current.showPreview).toBe(false);
  });
});
