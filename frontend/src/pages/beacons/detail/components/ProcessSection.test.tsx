import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import type { ProcessSnapshotMeta } from "@/lib/process-snapshot";
import ProcessSection from "./ProcessSection";

const { confirmMock } = vi.hoisted(() => ({ confirmMock: vi.fn() }));

vi.mock("@/lib/hooks/useConfirm", () => ({
  useConfirm: () => ({ confirm: confirmMock, modal: null }),
}));

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn() },
}));

vi.mock("@/lib/i18n", () => {
  // Real interpolation for templates the assertions read; every other key
  // resolves to itself. `t` must be referentially stable: action callbacks
  // carry it.
  const templates: Record<string, string> = {
    "agents.proc_action_queued": "Task {action} queued for PID {pid}",
    "agents.proc_batch_kill_queued": "{count} kill tasks queued",
  };
  const t = (key: string, params?: Record<string, string | number>) => {
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

const BASE_TEXT = "PID\tPPID\tUSER\tNAME\n1234\t0\talice\tnotepad.exe";
const BASE_SNAPSHOT: ProcessSnapshotMeta = {
  text: BASE_TEXT,
  source: "process_tree",
  live: false,
  kind: "process_tree",
  taskId: 10,
  collectedAt: "2026-10-21T12:00:00Z",
};
const TREE_TEXT = [
  "PID\tPPID\tUSER\tNAME",
  "1\t0\troot\tsystemd",
  "42\t1\troot\tsshd",
  "77\t42\talice\tbash",
].join("\n");

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  let reject: (reason?: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function renderSection(
  agentId = "a1",
  overrides: {
    snapshot?: ProcessSnapshotMeta | null;
    loading?: boolean;
    loadFailed?: boolean;
    collecting?: boolean;
    online?: boolean;
  } = {},
) {
  const onToggle = vi.fn();
  const onRefresh = vi.fn().mockResolvedValue(undefined);
  const onRetry = vi.fn();
  const utils = render(
    <ProcessSection
      agentId={agentId}
      online={overrides.online ?? true}
      snapshot={overrides.snapshot === undefined ? BASE_SNAPSHOT : overrides.snapshot}
      loading={overrides.loading ?? false}
      collecting={overrides.collecting ?? false}
      loadFailed={overrides.loadFailed ?? false}
      expanded
      onToggle={onToggle}
      onRetry={onRetry}
      onRefresh={onRefresh}
    />,
  );
  return { ...utils, onToggle, onRefresh, onRetry };
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

function rowAction(name: string, pid = "1234") {
  const row = screen.getByText(pid).closest("tr");
  if (!row) throw new Error("process row not found");
  return within(row).getByRole("button", { name });
}

function processRow(pid = "1234") {
  const row = screen.getByText(pid).closest("tr");
  if (!row) throw new Error("process row not found");
  return row;
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

    pending.resolve({ success: true });
    await act(async () => {
      await pending.promise;
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
    expect(successToast).not.toHaveBeenCalled();
  });

  it("clears the pid draft and snapshot-specific state when the operator switches agents", () => {
    const { rerender, onToggle, onRefresh } = renderSection("a1");
    fireEvent.change(pidInput(), { target: { value: "1234" } });
    fireEvent.click(screen.getAllByRole("checkbox")[1]);

    rerender(
      <ProcessSection
        agentId="a2"
        online
        snapshot={{ ...BASE_SNAPSHOT, taskId: 11 }}
        loading={false}
        collecting={false}
        loadFailed={false}
        expanded
        onToggle={onToggle}
        onRetry={vi.fn()}
        onRefresh={onRefresh}
      />,
    );

    expect((pidInput() as HTMLInputElement).value).toBe("");
    expect(screen.queryByText("agents.proc_selected")).toBeNull();
  });

  it("renders a real tree and collapses children on demand", () => {
    renderSection("a1", { snapshot: { ...BASE_SNAPSHOT, text: TREE_TEXT } });

    expect(screen.getAllByText("bash").length).toBeGreaterThan(0);
    expect(screen.getByText("systemd")).toBeDefined();

    fireEvent.click(screen.getAllByRole("button", { name: "agents.proc_collapse_branch" })[0]);
    expect(screen.queryByText("bash")).toBeNull();
    expect(screen.getByRole("button", { name: "agents.proc_expand_branch" })).toBeDefined();
  });

  it("shows process details without confusing USER for the process name", () => {
    renderSection();

    expect(screen.getByText("notepad.exe")).toBeDefined();
    expect(screen.queryByText("alice")).not.toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "agents.proc_details_for" }));
    expect(screen.getByText("agents.proc_raw")).toBeDefined();
    expect(screen.getByText("agents.proc_col_ppid")).toBeDefined();
  });

  it("blocks killing protected system processes", () => {
    renderSection("a1", {
      snapshot: {
        ...BASE_SNAPSHOT,
        text: "4\t0\tSYSTEM\tSystem",
      },
    });

    expect((rowAction("agents.proc_kill", "4") as HTMLButtonElement).disabled).toBe(true);
    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes[1].getAttribute("aria-disabled")).toBe("true");
    fireEvent.click(checkboxes[1]);
    expect(screen.queryByText("agents.proc_selected")).toBeNull();
  });

  it("selects eligible rows and queues a batch kill", async () => {
    renderSection("a1", {
      snapshot: {
        ...BASE_SNAPSHOT,
        text: ["PID\tPPID\tUSER\tNAME", "42\t1\troot\tsshd", "77\t42\talice\tbash"].join("\n"),
      },
    });

    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[1]);
    fireEvent.click(checkboxes[2]);
    expect(screen.getByText("agents.proc_selected")).toBeDefined();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "agents.proc_kill_selected" }));
    });

    expect(confirmMock).toHaveBeenCalledWith({ message: "agents.proc_batch_kill_confirm" });
    expect(api.post).toHaveBeenCalledTimes(2);
    expect(api.post).toHaveBeenCalledWith(paths.agents.cmd("a1", "killproc"), { target: "42" });
    expect(api.post).toHaveBeenCalledWith(paths.agents.cmd("a1", "killproc"), { target: "77" });
    await waitFor(() => expect(successToast).toHaveBeenCalledWith("2 kill tasks queued"));
  });

  it("distinguishes an empty snapshot from a load failure and can collect", async () => {
    const first = renderSection("a1", { snapshot: null });

    expect(screen.getByText("agents.proc_empty_title")).toBeDefined();
    await act(async () => {
      fireEvent.click(screen.getAllByRole("button", { name: "agents.proc_collect" })[0]);
    });
    expect(first.onRefresh).toHaveBeenCalledTimes(1);

    first.unmount();
    renderSection("a1", { snapshot: null, loadFailed: true });
    expect(screen.getByText("agents.proc_load_failed_msg")).toBeDefined();
  });

  it("shows collection status while retaining the current rows", () => {
    renderSection("a1", { collecting: true });
    expect(screen.getAllByText("agents.proc_collecting").length).toBeGreaterThan(0);
    expect(processRow()).toBeDefined();
    expect((screen.getByRole("button", { name: "agents.proc_collecting" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
