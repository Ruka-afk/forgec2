import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { AIContextPanel } from "./AIContextPanel";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), postFormData: vi.fn(), del: vi.fn(), putJson: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string, params?: Record<string, unknown>) =>
    params && typeof params.message === "string" ? `${key} · ${params.message}` : key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

const ATTACHMENT = { id: "att-1", filename: "run.log", size: 12 };

function renderPanel(overrides: Partial<Parameters<typeof AIContextPanel>[0]> = {}) {
  const props = {
    sessionId: 1 as number | null,
    profileId: null as number | null,
    selectedAttachmentIds: [] as string[],
    selectedCollectionIds: [] as number[],
    lowRiskAuto: false,
    onProfileChange: vi.fn(),
    onAttachmentIdsChange: vi.fn(),
    onCollectionIdsChange: vi.fn(),
    onLowRiskAutoChange: vi.fn(),
    ...overrides,
  };
  render(<AIContextPanel {...props} />);
  return props;
}

describe("AIContextPanel error surfaces", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps a failed context read onto the shared copy on both surfaces", async () => {
    vi.mocked(api.get).mockImplementation(async (path: unknown) => {
      const p = String(path);
      if (p === paths.ai.profiles) throw Object.assign(new Error("boom"), { status: 500 });
      return [];
    });
    renderPanel();

    await waitFor(() => expect(vi.mocked(toast.error)).toHaveBeenCalledWith("common.server_error"));
    expect(await screen.findByText("ai.context_unreadable · common.server_error")).toBeTruthy();
  });

  it("maps a failed attachment removal onto the shared status copy", async () => {
    vi.mocked(api.get).mockImplementation(async (path: unknown) => {
      const p = String(path);
      if (p === paths.ai.attachments(1)) return [ATTACHMENT];
      return [];
    });
    vi.mocked(api.del).mockRejectedValueOnce(Object.assign(new Error("boom"), { status: 500 }));
    renderPanel();

    fireEvent.click(await screen.findByRole("button", { name: "ai.attachment_remove" }));

    await waitFor(() => expect(vi.mocked(toast.error)).toHaveBeenCalledWith("common.server_error"));
  });

  it("maps a failed policy update and rolls the toggle back", async () => {
    vi.mocked(api.get).mockResolvedValue([]);
    vi.mocked(api.putJson).mockRejectedValueOnce(Object.assign(new Error("boom"), { status: 500 }));
    const props = renderPanel({ sessionId: 3 });

    fireEvent.click(await screen.findByRole("checkbox"));

    await waitFor(() => expect(vi.mocked(toast.error)).toHaveBeenCalledWith("common.server_error"));
    expect(props.onLowRiskAutoChange).toHaveBeenCalledWith(true);
    expect(props.onLowRiskAutoChange).toHaveBeenLastCalledWith(false);
    expect(vi.mocked(api.putJson)).toHaveBeenCalledWith(paths.ai.session(3), { write_policy: "low_risk_auto" });
  });

  it("maps a failed upload onto the shared status copy", async () => {
    vi.mocked(api.get).mockResolvedValue([]);
    vi.mocked(api.postFormData).mockRejectedValueOnce(Object.assign(new Error("boom"), { status: 500 }));
    renderPanel({ sessionId: 5 });

    const input = document.querySelector<HTMLInputElement>('input[type="file"]');
    if (!input) throw new Error("file input not rendered");
    const file = new File(["log"], "run.log", { type: "text/plain" });
    Object.defineProperty(input, "files", { value: [file], configurable: true });
    fireEvent.change(input);

    await waitFor(() => expect(vi.mocked(toast.error)).toHaveBeenCalledWith("common.server_error"));
    expect(vi.mocked(api.postFormData)).toHaveBeenCalledWith(
      paths.ai.attachments(5),
      expect.anything(),
      expect.objectContaining({ timeout: 60000 }),
    );
  });
});
