import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { AgentDockCommands } from "./AgentDockCommands";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn(), postJson: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  // Must be referentially stable: the mutations' callbacks carry it.
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn() }),
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);

function renderCommands(onQueued = vi.fn()) {
  const view = render(<AgentDockCommands agentId="a1" onQueued={onQueued} />);
  return { view, onQueued };
}

async function startedSocksButton() {
  // The status probe resolves "not active" → the button offers "start".
  return screen.findByRole("button", { name: "agents.dock_cmd_socks" });
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.get).mockResolvedValue({ active: false, port: null });
});

describe("AgentDockCommands", () => {
  it("surfaces the backend reason when a queued command is rejected and clears busy", async () => {
    renderCommands();
    const sleep = screen.getByRole("button", { name: "agents.dock_cmd_sleep" });

    vi.mocked(api.postJson).mockRejectedValue({ status: 400, message: "sleep out of range" });
    await act(async () => {
      fireEvent.click(sleep);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("sleep out of range"));
    expect((sleep as HTMLButtonElement).disabled).toBe(false);
  });

  it("queues a sleep command and hands the task to the dock", async () => {
    const { onQueued } = renderCommands();
    const sleep = screen.getByRole("button", { name: "agents.dock_cmd_sleep" });

    vi.mocked(api.postJson).mockResolvedValue({ task_id: 7 });
    await act(async () => {
      fireEvent.click(sleep);
    });

    await waitFor(() => expect(successToast).toHaveBeenCalledWith("agents.dock_cmd_queued"));
    expect(onQueued).toHaveBeenCalledWith({ task_id: 7, type: "set_sleep", command: "60,10" });
    expect(api.postJson).toHaveBeenCalledWith(paths.agents.setSleep("a1"), { interval: 60, jitter: 10 });
  });

  it("normalizes a failed SOCKS start into the relay message", async () => {
    renderCommands();
    const socks = await startedSocksButton();

    vi.mocked(api.post).mockRejectedValue({ status: 400, message: "port already taken" });
    await act(async () => {
      fireEvent.click(socks);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("port already taken"));
    expect((socks as HTMLButtonElement).disabled).toBe(false);
  });

  it("keeps the interval validation toast direct", async () => {
    renderCommands();
    const input = screen.getByRole("textbox", { name: "agents.dock_cmd_sleep" });
    fireEvent.change(input, { target: { value: "nonsense" } });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "agents.dock_cmd_sleep" }));
    });

    expect(errorToast).toHaveBeenCalledWith("agents.dock_cmd_sleep_invalid");
    expect(api.postJson).not.toHaveBeenCalled();
  });
});
