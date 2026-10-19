import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import NLQueryDialog from "./NLQueryDialog";

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

const postJsonMock = vi.mocked(api.postJson);
const errorToast = vi.mocked(toast.error);

async function submitQuestion(question: string) {
  fireEvent.click(screen.getByRole("button", { name: "ai.nl_button" }));
  const input = await screen.findByPlaceholderText("ai.nl_placeholder");
  fireEvent.change(input, { target: { value: question } });
  // Keep the already-rejected request's continuation (and its loading-state
  // update) inside an act window instead of after the event has returned.
  await act(async () => {
    fireEvent.keyDown(input, { key: "Enter" });
  });
}

describe("NLQueryDialog error surfaces", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps a failed query onto the shared status copy instead of the raw message", async () => {
    postJsonMock.mockRejectedValueOnce(Object.assign(new Error("boom"), { status: 500 }));
    render(<NLQueryDialog />);

    await submitQuestion("which hosts ran recon");

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
    expect(postJsonMock).toHaveBeenCalledWith(paths.ai.nlQuery, { question: "which hosts ran recon" });
  });

  it("keeps the backend validation text for a rejected question", async () => {
    postJsonMock.mockRejectedValueOnce(Object.assign(new Error("question too long"), { status: 400 }));
    render(<NLQueryDialog />);

    await submitQuestion("q");

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("question too long"));
  });
});
