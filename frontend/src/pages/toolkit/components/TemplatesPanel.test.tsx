import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import TemplatesPanel from "./TemplatesPanel";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), postJson: vi.fn(), putJson: vi.fn(), del: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

const confirmMock = vi.hoisted(() => vi.fn(async () => true));
vi.mock("@/lib/hooks/useConfirm", () => ({
  useConfirm: () => ({ confirm: confirmMock, modal: null }),
}));

vi.mock("@/lib/hooks/usePermissions", () => ({
  usePermissions: () => ({ can: () => true }),
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);

const TEMPLATE = { id: 1, name: "recon-1", category: "recon", command: "whoami" };

async function openCreateDialog() {
  fireEvent.click(screen.getAllByRole("button", { name: "templates.create" })[0]);
  fireEvent.change(await screen.findByLabelText("common.name"), { target: { value: "t1" } });
  fireEvent.change(screen.getByLabelText("common.command"), { target: { value: "whoami" } });
}

beforeEach(() => {
  vi.clearAllMocks();
  confirmMock.mockResolvedValue(true);
});

describe("TemplatesPanel", () => {
  it("reports a failed load inline and recovers on retry", async () => {
    vi.mocked(api.get).mockRejectedValueOnce({ status: 500, message: "boom" });
    render(<TemplatesPanel />);

    expect(await screen.findByText("common.server_error")).toBeTruthy();
    // The empty-state must not read as "no templates" while the load failed.
    expect(screen.queryByText("templates.empty_title")).toBeNull();

    vi.mocked(api.get).mockResolvedValue({ templates: [TEMPLATE] });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "common.retry" }));
    });

    expect(await screen.findByText("recon-1")).toBeTruthy();
    expect(screen.queryByText("common.server_error")).toBeNull();
  });

  it("surfaces the backend reason when saving is rejected and keeps the editor open", async () => {
    vi.mocked(api.get).mockResolvedValue({ templates: [] });
    render(<TemplatesPanel />);
    await screen.findByText("templates.empty_title");

    await openCreateDialog();
    vi.mocked(api.postJson).mockRejectedValue({ status: 400, message: "name taken" });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "common.save" }));
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("name taken"));
    expect(screen.getByLabelText("common.name")).toBeTruthy();
  });

  it("reports a saved template and reloads the list", async () => {
    vi.mocked(api.get).mockResolvedValue({ templates: [] });
    render(<TemplatesPanel />);
    await screen.findByText("templates.empty_title");

    await openCreateDialog();
    vi.mocked(api.postJson).mockResolvedValue({ success: true });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "common.save" }));
    });

    await waitFor(() => expect(successToast).toHaveBeenCalledWith("templates.toast.created"));
    expect(api.get).toHaveBeenCalledTimes(2);
  });

  it("resolves a forbidden template run through the normalizer", async () => {
    vi.mocked(api.get).mockResolvedValue({ templates: [TEMPLATE] });
    render(<TemplatesPanel selectedAgent="a1" />);
    await screen.findByText("recon-1");

    vi.mocked(api.postJson).mockRejectedValue({ status: 403, message: "denied" });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "templates.run" }));
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.forbidden"));
    expect(api.postJson).toHaveBeenCalledWith(paths.agents.command("a1"), { command: "whoami" });
  });

  it("keeps the no-agent run guard a direct toast", async () => {
    vi.mocked(api.get).mockResolvedValue({ templates: [TEMPLATE] });
    render(<TemplatesPanel />);
    await screen.findByText("recon-1");

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "templates.run" }));
    });

    expect(errorToast).toHaveBeenCalledWith("toolkit.toast.select_agent_first");
    expect(api.postJson).not.toHaveBeenCalled();
  });

  it("surfaces the backend reason when a delete is rejected", async () => {
    vi.mocked(api.get).mockResolvedValue({ templates: [TEMPLATE] });
    render(<TemplatesPanel />);
    await screen.findByText("recon-1");

    vi.mocked(api.del).mockRejectedValue({ status: 409, message: "referenced by a run" });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "common.delete" }));
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("referenced by a run"));
  });
});
