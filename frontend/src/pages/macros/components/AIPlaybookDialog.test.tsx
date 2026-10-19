import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { AIPlaybookDialog } from "./AIPlaybookDialog";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), postJson: vi.fn(), putJson: vi.fn(), del: vi.fn(), postFormData: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

const aiStatusMock = vi.hoisted(() => ({
  status: { enabled: true, hasApiKey: true, known: true },
  reload: vi.fn(),
}));
vi.mock("@/lib/hooks/useAIStatus", () => ({ useAIStatus: () => aiStatusMock }));

vi.mock("@/lib/agents", () => ({
  fetchAgentListCached: vi.fn(async () => [{ id: "agent-1", hostname: "WINBOX" }]),
}));

const postJsonMock = vi.mocked(api.postJson);
const errorToast = vi.mocked(toast.error);

const DRAFT = { name: "PB", description: "", steps: [{ command: "whoami", shell: "cmd" }] };

async function openAndGenerate(goal = "recon the subnet") {
  // The dialog open path loads the agent list asynchronously; keep that
  // settlement (and the generate request's continuation below) inside act.
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "ai.pb_button" }));
  });
  const input = await screen.findByPlaceholderText("ai.pb_goal_placeholder");
  fireEvent.change(input, { target: { value: goal } });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "ai.pb_generate_btn" }));
  });
}

describe("AIPlaybookDialog error surfaces", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps a failed generation onto the shared status copy instead of the raw message", async () => {
    postJsonMock.mockRejectedValueOnce(Object.assign(new Error("boom"), { status: 500 }));
    render(<AIPlaybookDialog onSaved={() => {}} />);

    await openAndGenerate();

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
    expect(postJsonMock).toHaveBeenCalledWith(paths.ai.generatePlaybook, { goal: "recon the subnet", agent_id: "" });
  });

  it("maps a failed save onto the shared status copy instead of the raw message", async () => {
    postJsonMock.mockResolvedValueOnce(DRAFT);
    postJsonMock.mockRejectedValueOnce(Object.assign(new Error("boom"), { status: 500 }));
    render(<AIPlaybookDialog onSaved={() => {}} />);

    await openAndGenerate();
    const saveBtn = await screen.findByRole("button", { name: "ai.pb_save_btn" });
    await act(async () => {
      fireEvent.click(saveBtn);
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
    expect(postJsonMock).toHaveBeenLastCalledWith(paths.ai.savePlaybook, DRAFT);
  });

  it("keeps the backend validation text for a rejected generation", async () => {
    postJsonMock.mockRejectedValueOnce(Object.assign(new Error("goal refused by policy"), { status: 422 }));
    render(<AIPlaybookDialog onSaved={() => {}} />);

    await openAndGenerate();

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("goal refused by policy"));
  });
});
