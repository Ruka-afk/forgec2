import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import AIAnalysisButton from "./AIAnalysisPanel";

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

function serverError(message = "boom") {
  return Object.assign(new Error(message), { status: 500 });
}

describe("AIAnalysisPanel error surfaces", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps a failed analysis onto the shared status copy instead of the raw message", async () => {
    postJsonMock.mockRejectedValueOnce(serverError());
    render(<AIAnalysisButton taskId={7} />);

    fireEvent.click(screen.getByRole("button", { name: "ai.analyze_button" }));

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
    expect(postJsonMock).toHaveBeenCalledWith(paths.ai.analyzeResult, { task_id: 7 });
  });

  it("keeps the backend validation text for a rejected analysis", async () => {
    postJsonMock.mockRejectedValueOnce(Object.assign(new Error("task not found"), { status: 404 }));
    render(<AIAnalysisButton taskId={7} />);

    fireEvent.click(screen.getByRole("button", { name: "ai.analyze_button" }));

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("task not found"));
  });

  it("still reports a success response that carries no analysis", async () => {
    postJsonMock.mockResolvedValueOnce({});
    render(<AIAnalysisButton taskId={7} />);

    fireEvent.click(screen.getByRole("button", { name: "ai.analyze_button" }));

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("ai.toast.assist_failed"));
  });
});
