import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import ProcessSection from "./ProcessSection";

const { confirmMock } = vi.hoisted(() => ({ confirmMock: vi.fn() }));

vi.mock("@/lib/hooks/useConfirm", () => ({
  useConfirm: () => ({ confirm: confirmMock, modal: null }),
}));

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn() },
}));

vi.mock("@/lib/i18n", () => {
  // Real interpolation for the queued-toast template; every other key
  // resolves to itself. `t` must be referentially stable: the mutation's
  // callbacks carry it.
  const templates: Record<string, string> = {
    "agents.proc_action_queued": "Task {action} queued for PID {pid}",
  };
  const t = (key: string, params?: Record<string, string>) => {
    let s = templates[key] ?? key;
    if (params && typeof params === "object") {
      for (const [k, v] of Object.entries(params)) s = s.replace(`{${k}}`, String(v));
    }
    return s;
  };
  return { useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  let reject: (reason?: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function renderSection(agentId = "a1") {
  const onToggle = vi.fn();
  const onRefresh = vi.fn();
  const utils = render(
    <ProcessSection
      agentId={agentId}
      online
      processList={"1234 notepad.exe"}
      loading={false}
      loadFailed={false}
      expanded
      onToggle={onToggle}
      onRefresh={onRefresh}
    />,
  );
  return { ...utils, onToggle, onRefresh };
}

function pidInput() {
  return screen.getByPlaceholderText("agents.proc_pid_ph");
}

/** The manual action buttons share their labels with the row icon buttons. */
function manualAction(name: string) {
  const bar = pidInput().parentElement;
  if (!bar) throw new Error("manual action bar not found");
  return within(bar).getByRole("button", { name });
}

function rowAction(name: string) {
  const row = screen.getByText("1234").closest("tr");
  if (!row) throw new Error("process row not found");
  return within(row).getByRole("button", { name });
}

beforeEach(() => {
  vi.clearAllMocks();
  confirmMock.mockResolvedValue(true);
  vi.mocked(api.post).mockResolvedValue({ success: true });
});

describe("ProcessSection", () => {
  it("queues a manual suspend without confirmation", async () => {
    renderSection();
    fireEvent.change(pidInput(), { target: { value: "1234" } });

    await act(async () => {
      fireEvent.click(manualAction("agents.proc_suspend"));
    });

    expect(confirmMock).not.toHaveBeenCalled();
    expect(api.post).toHaveBeenCalledWith(paths.agents.cmd("a1", "suspend"), { target: "1234" });
    await waitFor(() => expect(successToast).toHaveBeenCalledWith("Task suspend queued for PID 1234"));
  });

  it("keeps the controls disabled while an action is in flight", async () => {
    const pending = deferred<Record<string, unknown>>();
    vi.mocked(api.post).mockImplementation(() => pending.promise);
    renderSection();
    fireEvent.change(pidInput(), { target: { value: "1234" } });

    fireEvent.click(manualAction("agents.proc_suspend"));

    const suspend = manualAction("agents.proc_suspend");
    await waitFor(() => expect((suspend as HTMLButtonElement).disabled).toBe(true));

    await act(async () => {
      pending.resolve({ success: true });
    });
    await waitFor(() => expect((suspend as HTMLButtonElement).disabled).toBe(false));
  });

  it("requires the destructive confirm for a row kill and aborts when dismissed", async () => {
    confirmMock.mockResolvedValue(false);
    renderSection();

    await act(async () => {
      fireEvent.click(rowAction("agents.proc_kill"));
    });
    expect(confirmMock).toHaveBeenCalledWith({ message: "agents.proc_kill_confirm" });
    expect(api.post).not.toHaveBeenCalled();

    confirmMock.mockResolvedValue(true);
    await act(async () => {
      fireEvent.click(rowAction("agents.proc_kill"));
    });
    expect(api.post).toHaveBeenCalledWith(paths.agents.cmd("a1", "killproc"), { target: "1234" });
  });

  it("surfaces the backend message when an action is rejected", async () => {
    vi.mocked(api.post).mockRejectedValue({ status: 400, message: "access denied" });
    renderSection();
    fireEvent.change(pidInput(), { target: { value: "1234" } });

    await act(async () => {
      fireEvent.click(manualAction("agents.proc_suspend"));
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("access denied"));
  });

  it("clears the pid draft when the operator switches agents", async () => {
    const { rerender, onToggle } = renderSection("a1");
    fireEvent.change(pidInput(), { target: { value: "1234" } });

    rerender(
      <ProcessSection
        agentId="a2"
        online
        processList={"1234 notepad.exe"}
        loading={false}
        loadFailed={false}
        expanded
        onToggle={onToggle}
        onRefresh={vi.fn()}
      />,
    );

    expect((pidInput() as HTMLInputElement).value).toBe("");
  });
});
