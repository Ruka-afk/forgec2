import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { pullRemoteFile, pushLocalFile } from "@/lib/agent-files/file-transfer";
import { AgentDockFiles } from "./AgentDockFiles";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn(), postJson: vi.fn() },
  pollTask: vi.fn(),
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/agent-files/file-transfer", () => ({
  pullRemoteFile: vi.fn(),
  pushLocalFile: vi.fn(),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }),
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);

function listing() {
  return {
    files: [
      { name: "a.txt", is_dir: false, size: 10, mod_time: "" },
      { name: "sub", is_dir: true, size: 0, mod_time: "" },
    ],
  };
}

function renderDock() {
  return render(
    <MemoryRouter>
      <AgentDockFiles agentId="a1" osType="linux" />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.post).mockResolvedValue(listing());
});

describe("AgentDockFiles", () => {
  it("renders the normalized load failure inline instead of the raw message", async () => {
    vi.mocked(api.post).mockRejectedValue({ status: 500, message: "boom" });

    renderDock();

    await waitFor(() => expect(screen.getByText("common.server_error")).toBeTruthy());
    expect(errorToast).not.toHaveBeenCalled();
  });

  it("normalizes a failed pull into its message and clears the row busy", async () => {
    renderDock();
    await screen.findByText("a.txt");

    vi.mocked(pullRemoteFile).mockRejectedValue({ status: 500, message: "boom" });
    const download = screen.getByRole("button", { name: "agents.files_pull" });
    await act(async () => {
      fireEvent.click(download);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
    expect(toast).toHaveBeenCalledWith("agents.files_pull_queued");
    expect((download as HTMLButtonElement).disabled).toBe(false);
  });

  it("reports a completed pull with the partial/full copy", async () => {
    renderDock();
    await screen.findByText("a.txt");

    vi.mocked(pullRemoteFile).mockResolvedValue("pulled");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "agents.files_pull" }));
    });

    await waitFor(() => expect(successToast).toHaveBeenCalledWith("agents.files_pulled"));
  });

  it("normalizes a failed push into its message", async () => {
    renderDock();
    await screen.findByText("a.txt");

    vi.mocked(pushLocalFile).mockRejectedValue({ status: 500, message: "boom" });
    const input = screen.getByLabelText("agents.files.select_upload") as HTMLInputElement;
    const file = new File(["x"], "up.txt");
    Object.defineProperty(input, "files", { value: [file], configurable: true });
    await act(async () => {
      fireEvent.change(input);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
    expect(pushLocalFile).toHaveBeenCalledWith(expect.objectContaining({ destPath: expect.stringContaining("up.txt") }));
  });
});
