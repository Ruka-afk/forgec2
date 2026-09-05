import { describe, it, expect, vi, beforeEach } from "vitest";
import { collectTaskResult, base64ToBytes } from "./task-collect";
import { api, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";

vi.mock("@/lib/api", () => ({
  api: {
    post: vi.fn(),
    get: vi.fn(),
  },
  pollTask: vi.fn(),
}));

const mockedPost = vi.mocked(api.post);
const mockedGet = vi.mocked(api.get);
const mockedPoll = vi.mocked(pollTask);

beforeEach(() => {
  vi.clearAllMocks();
});

function pollOk() {
  mockedPoll.mockResolvedValue({ status: "completed", result: "x" } as never);
}

describe("collectTaskResult", () => {
  it("returns the task result after polling", async () => {
    mockedPost.mockResolvedValue({ task_id: 7 });
    pollOk();
    mockedGet.mockResolvedValue({ result: "hello" });

    const out = await collectTaskResult("a1", paths.agents.drives("a1"));

    expect(out).toBe("hello");
    expect(mockedPost).toHaveBeenCalledWith(paths.agents.drives("a1"), {});
    expect(mockedPoll).toHaveBeenCalledWith("a1", 7, { timeoutMs: 120_000 });
    expect(mockedGet).toHaveBeenCalledWith(paths.agents.task("a1", "7"));
  });

  it("falls back to data.result and forwards body/timeout", async () => {
    mockedPost.mockResolvedValue({ task_id: 9 });
    pollOk();
    mockedGet.mockResolvedValue({ data: { result: "nested" } });

    const out = await collectTaskResult("a1", "/p", { target: "t" }, 5000);

    expect(out).toBe("nested");
    expect(mockedPost).toHaveBeenCalledWith("/p", { target: "t" });
    expect(mockedPoll).toHaveBeenCalledWith("a1", 9, { timeoutMs: 5000 });
  });

  it("throws when dispatch yields no task id", async () => {
    mockedPost.mockResolvedValue({});

    await expect(collectTaskResult("a1", "/p")).rejects.toThrow("no task id");
    expect(mockedPoll).not.toHaveBeenCalled();
  });

  it("throws the poll error when the task fails", async () => {
    mockedPost.mockResolvedValue({ task_id: 3 });
    mockedPoll.mockResolvedValue({ status: "failed", error: "boom" } as never);

    await expect(collectTaskResult("a1", "/p")).rejects.toThrow("boom");
    expect(mockedGet).not.toHaveBeenCalled();
  });

  it("throws a default error when the task fails without detail", async () => {
    mockedPost.mockResolvedValue({ task_id: 3 });
    mockedPoll.mockResolvedValue({ status: "failed" } as never);

    await expect(collectTaskResult("a1", "/p")).rejects.toThrow("task failed");
  });
});

describe("base64ToBytes", () => {
  it("decodes base64 with whitespace to bytes", () => {
    const out = base64ToBytes("aGVs\nbG8=");
    expect(Array.from(out)).toEqual([104, 101, 108, 108, 111]);
  });
});
