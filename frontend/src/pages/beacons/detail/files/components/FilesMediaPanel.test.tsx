import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { api, pollTask } from "@/lib/api";
import type { FileSearchHit } from "@/lib/agent-files/file-search";
import FilesMediaPanel from "./FilesMediaPanel";

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn() },
  pollTask: vi.fn(),
  getRateLimitRetryAfter: vi.fn(() => 0),
}));

const t = (key: string) => key;

function hit(overrides: Partial<FileSearchHit> = {}): FileSearchHit {
  return {
    path: "C:\\Users\\a\\pic.jpg",
    name: "pic.jpg",
    ext: "jpg",
    category: "image",
    size: 1234,
    modified: "2026-10-20T10:00:00+08:00",
    ...overrides,
  };
}

function resultPayload(overrides: Record<string, unknown> = {}) {
  return JSON.stringify({
    root: "C:\\Users\\a",
    hits: [hit()],
    scanned: 9,
    errors: 0,
    truncated: false,
    elapsed_ms: 12,
    ...overrides,
  });
}

beforeEach(() => {
  vi.mocked(api.post).mockReset();
  vi.mocked(pollTask).mockReset();
  vi.mocked(api.post).mockResolvedValue({ success: true, task_id: 7 });
});

describe("FilesMediaPanel", () => {
  it("queues a metadata search and renders the returned hits", async () => {
    vi.mocked(pollTask).mockResolvedValue({ status: "completed", id: 7, result: resultPayload() });
    const onReveal = vi.fn();
    render(<FilesMediaPanel t={t} agentId="a1" rootPath={"C:\\Users\\a"} onReveal={onReveal} />);

    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_search" }));

    await waitFor(() => expect(screen.getByText("pic.jpg")).toBeTruthy());
    expect(api.post).toHaveBeenCalledWith(
      "/agents/a1/files/search",
      expect.objectContaining({
        path: "C:\\Users\\a",
        pattern: "*",
        categories: "image,video,audio",
        max_depth: "8",
        include_hidden: "0",
      }),
    );
    expect(pollTask).toHaveBeenCalledWith("a1", 7, expect.objectContaining({ timeoutMs: 180_000 }));

    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_reveal" }));
    expect(onReveal).toHaveBeenCalledWith(hit());
  });

  it("sends only the selected category when the All chip is replaced", async () => {
    vi.mocked(pollTask).mockResolvedValue({ status: "completed", id: 7, result: resultPayload() });
    render(<FilesMediaPanel t={t} agentId="a1" rootPath="/home/a" onReveal={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_all" }));
    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_image" }));
    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_search" }));

    await waitFor(() => expect(api.post).toHaveBeenCalled());
    expect(api.post).toHaveBeenCalledWith(
      "/agents/a1/files/search",
      expect.objectContaining({ categories: "image" }),
    );
  });

  it("surfaces truncated results and unreadable paths", async () => {
    vi.mocked(pollTask).mockResolvedValue({
      status: "completed",
      id: 7,
      result: resultPayload({ truncated: true, truncated_reason: "max_results", errors: 3 }),
    });
    render(<FilesMediaPanel t={t} agentId="a1" rootPath={"C:\\Users\\a"} onReveal={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_search" }));

    await waitFor(() => expect(screen.getByText(/agents\.files_media_truncated/)).toBeTruthy());
    expect(screen.getByText(/agents\.files_media_trunc_max_results/)).toBeTruthy();
    expect(screen.getByText("agents.files_media_errors")).toBeTruthy();
  });

  it("shows task failures and does not leave the button in a loading state", async () => {
    vi.mocked(pollTask).mockResolvedValue({ status: "failed", id: 7, error: "search exploded" });
    render(<FilesMediaPanel t={t} agentId="a1" rootPath={"C:\\Users\\a"} onReveal={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_search" }));

    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByText("search exploded")).toBeTruthy();
    expect(screen.getByRole("button", { name: "agents.files_media_search" })).toBeTruthy();
  });

  it("rejects an ACK instead of polling when the server does not return a task id", async () => {
    vi.mocked(api.post).mockResolvedValue({ success: true });
    render(<FilesMediaPanel t={t} agentId="a1" rootPath={"C:\\Users\\a"} onReveal={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_search" }));

    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByText("agents.files_media_bad_ack")).toBeTruthy();
    expect(pollTask).not.toHaveBeenCalled();
  });

  it("loads bounded thumbnails for image hits in the default grid", async () => {
    const thumbId = "a".repeat(64);
    vi.mocked(api.post).mockImplementation((path: string) =>
      Promise.resolve(path.endsWith("/thumb") ? { task_id: 8 } : { task_id: 7 }),
    );
    vi.mocked(pollTask).mockImplementation((_agentId, taskId) =>
      Promise.resolve(
        taskId === 8
          ? {
              status: "completed",
              id: 8,
              result: JSON.stringify({
                items: [
                  {
                    path: "C:\\Users\\a\\pic.jpg",
                    ok: true,
                    thumb_id: thumbId,
                    width: 320,
                    height: 200,
                  },
                ],
              }),
            }
          : { status: "completed", id: 7, result: resultPayload() },
      ),
    );
    const { container } = render(
      <FilesMediaPanel t={t} agentId="a1" rootPath={"C:\\Users\\a"} onReveal={vi.fn()} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_search" }));

    await waitFor(() =>
      expect(container.querySelector("img")?.getAttribute("src")).toBe(
        `/agents/a1/files/thumb/${thumbId}`,
      ),
    );
    expect(api.post).toHaveBeenCalledWith("/agents/a1/files/thumb", {
      paths: JSON.stringify(["C:\\Users\\a\\pic.jpg"]),
    });
  });

  it("refuses playback above the transfer cap without queuing an exfil", async () => {
    vi.mocked(pollTask).mockResolvedValue({
      status: "completed",
      id: 7,
      result: resultPayload({
        hits: [hit({ category: "video", name: "movie.mp4", path: "C:\\Users\\a\\movie.mp4", size: 50 * 1024 * 1024 + 1 })],
      }),
    });
    render(<FilesMediaPanel t={t} agentId="a1" rootPath={"C:\\Users\\a"} onReveal={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_search" }));
    await waitFor(() => expect(screen.getByText("movie.mp4")).toBeTruthy());
    fireEvent.click(screen.getByRole("button", { name: "agents.files_media_play" }));

    await waitFor(() => expect(screen.getByText("agents.files_media_play_too_large")).toBeTruthy());
    expect(api.post).not.toHaveBeenCalledWith("/agents/a1/files/pull", expect.anything());
  });
});
