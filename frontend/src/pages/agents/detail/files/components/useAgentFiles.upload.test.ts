import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useAgentFiles } from "./useAgentFiles";
import { pushLocalFile } from "./file-transfer";

vi.mock("./file-transfer", () => ({
  pushLocalFile: vi.fn(),
  pullRemoteFile: vi.fn(),
}));

// Stable t identity mirrors the real useI18n (useCallback): a fresh arrow
// per render would invalidate loadDirectory and abort uploads on every
// setState via the mount effect cleanup.
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

const mockedPush = vi.mocked(pushLocalFile);

function deferred() {
  let resolve!: () => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<void>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

const fileA = new File(["a"], "a.txt");
const fileB = new File(["b"], "b.txt");

beforeEach(() => {
  vi.clearAllMocks();
});

describe("useAgentFiles upload queue", () => {
  it("serializes concurrent uploads: second starts after first settles", async () => {
    const first = deferred();
    const second = deferred();
    mockedPush.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);

    const { result } = renderHook(() => useAgentFiles("agent-1"));

    act(() => {
      result.current.uploadFile(fileA);
      result.current.uploadFile(fileB);
    });
    expect(mockedPush).toHaveBeenCalledTimes(1);

    await act(async () => {
      first.resolve();
    });
    expect(mockedPush).toHaveBeenCalledTimes(2);

    await act(async () => {
      second.resolve();
    });
  });

  it("cancel aborts in-flight and drops queued files", async () => {
    const first = deferred();
    mockedPush.mockReturnValue(first.promise);

    const { result } = renderHook(() => useAgentFiles("agent-1"));

    act(() => {
      result.current.uploadFile(fileA);
      result.current.uploadFile(fileB);
    });
    expect(mockedPush).toHaveBeenCalledTimes(1);

    act(() => {
      result.current.cancelUpload();
    });
    await act(async () => {
      first.resolve();
    });
    // Queued file B never started.
    expect(mockedPush).toHaveBeenCalledTimes(1);
  });
});
