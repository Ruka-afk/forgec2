import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { useAppStore } from "@/lib/store";
import { AgentInteractDock } from "./AgentInteractDock";
import type { Beacon } from "./types";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn(), postJson: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  // Must be referentially stable: the load effect depends on it.
  const t = (key: string) => key;
  return {
    getLocale: () => "en",
    useI18n: () => ({ t }),
    I18nProvider: ({ children }: { children: ReactNode }) => children,
  };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

vi.mock("@/lib/wsContext", () => ({
  useWS: () => ({ subscribe: vi.fn(() => () => {}), connected: true }),
}));

// The lazy children own their own suites; stub them so this test only drives
// the dock's own load/review paths.
vi.mock("./AgentDockFiles", () => ({ AgentDockFiles: () => null }));
vi.mock("./AgentDockCommands", () => ({ AgentDockCommands: () => null }));
vi.mock("./AgentDockShot", () => ({ AgentDockShot: () => null }));
vi.mock("@/components/ShellTerminal", () => ({ default: () => null }));

const errorToast = vi.mocked(toast.error);

const beacon = { id: "a1", hostname: "host1", status: "online" } as Beacon;

function task(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    type: "shell",
    command: "whoami",
    status: "pending_approval",
    created_by: "alice",
    created_at: "",
    ...overrides,
  };
}

function renderDock() {
  return render(
    <MemoryRouter>
      <AgentInteractDock
        beacon={beacon}
        height={320}
        pinned={false}
        tab="tasks"
        onTabChange={vi.fn()}
        onHeightChange={vi.fn()}
        onTogglePin={vi.fn()}
        onClose={vi.fn()}
        onOpenDetails={vi.fn()}
      />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  useAppStore.setState({ currentUsername: "bob" });
});

describe("AgentInteractDock", () => {
  it("surfaces the backend reason when an approval is rejected", async () => {
    vi.mocked(api.get)
      .mockResolvedValueOnce({ tasks: [task()] })
      .mockResolvedValueOnce(task());
    renderDock();

    await screen.findByText("whoami");
    fireEvent.click(screen.getByText("whoami"));
    const approve = await screen.findByRole("button", { name: "agents.dock_task_approve" });

    vi.mocked(api.post).mockRejectedValue({ status: 400, message: "two-man rule" });
    await act(async () => {
      fireEvent.click(approve);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("two-man rule"));
  });

  it("surfaces the backend reason when a cancel is rejected", async () => {
    vi.mocked(api.get)
      .mockResolvedValueOnce({ tasks: [task({ status: "running" })] })
      .mockResolvedValueOnce(task({ status: "running" }));
    renderDock();

    await screen.findByText("whoami");
    fireEvent.click(screen.getByText("whoami"));
    const cancel = await screen.findByRole("button", { name: "agents.dock_task_cancel" });

    vi.mocked(api.post).mockRejectedValue({ status: 500, message: "boom" });
    await act(async () => {
      fireEvent.click(cancel);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
  });

  it("shows the normalized inline error with a retry when the first load fails", async () => {
    vi.mocked(api.get)
      .mockRejectedValueOnce({ status: 500, message: "boom" })
      .mockResolvedValueOnce({ tasks: [task()] });
    renderDock();

    await screen.findByText("common.server_error");
    // The misleading "no tasks" copy must not render behind the error.
    expect(screen.queryByText("agents.dock_no_tasks")).toBeNull();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "agents.detail_retry" }));
    });

    await screen.findByText("whoami");
  });
});
