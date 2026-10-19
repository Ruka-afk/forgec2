import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { AIAssistBar } from "./AIAssistBar";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), postJson: vi.fn() },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

const getMock = vi.mocked(api.get);
const postJsonMock = vi.mocked(api.postJson);
const errorToast = vi.mocked(toast.error);

function renderBar(sessionId: number | null = 7) {
  return render(<AIAssistBar sessionId={sessionId} onInsertPrompt={vi.fn()} />);
}

describe("AIAssistBar error surfaces", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getMock.mockResolvedValue([]);
  });

  it("maps a failed run review onto the shared status copy", async () => {
    postJsonMock.mockRejectedValueOnce(Object.assign(new Error("boom"), { status: 500 }));
    renderBar();
    const review = screen.getByRole("button", { name: "ai.review_run" });
    await waitFor(() => expect(getMock).toHaveBeenCalledWith(paths.macros.list, expect.anything()));
    errorToast.mockClear();

    fireEvent.click(review);

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
    expect(postJsonMock).toHaveBeenCalledWith(paths.ai.runReview, { session_id: 7 }, expect.anything());
  });

  it("keeps the backend validation text for a rejected review request", async () => {
    postJsonMock.mockRejectedValueOnce(Object.assign(new Error("no finished run"), { status: 400 }));
    renderBar();
    await waitFor(() => expect(getMock).toHaveBeenCalled());
    errorToast.mockClear();

    fireEvent.click(screen.getByRole("button", { name: "ai.review_run" }));

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("no finished run"));
  });
});
