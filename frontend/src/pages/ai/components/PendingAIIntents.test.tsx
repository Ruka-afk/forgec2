import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { PendingAIIntents } from "./PendingAIIntents";

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

const INTENT = {
  id: "i-1",
  session_id: 1,
  tool_name: "shell_exec",
  risk: "write",
  target_summary: "host-1",
  arguments_summary: '{ "cmd": "whoami" }',
  status: "pending",
  created_at: "2026-10-01T10:00:00.000Z",
};

describe("PendingAIIntents error surfaces", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.get).mockResolvedValue({ intents: [INTENT] });
  });

  it("maps a failed approval onto the shared status copy and re-enables the buttons", async () => {
    vi.mocked(api.postJson).mockRejectedValueOnce(Object.assign(new Error("boom"), { status: 500 }));
    render(<PendingAIIntents activeSessionId={null} />);

    const approve = await screen.findByRole("button", { name: "ai.intent_approve_execute" });
    vi.mocked(toast.error).mockClear();
    fireEvent.click(approve);

    await waitFor(() => expect(vi.mocked(toast.error)).toHaveBeenCalledWith("common.server_error"));
    expect(vi.mocked(api.postJson)).toHaveBeenCalledWith(paths.ai.intentApprove("i-1"), {});
    await waitFor(() => {
      const button = screen.getByRole("button", { name: "ai.intent_approve_execute" }) as HTMLButtonElement;
      expect(button.disabled).toBe(false);
    });
  });

  it("keeps the backend validation text for a rejected decision", async () => {
    vi.mocked(api.postJson).mockRejectedValueOnce(Object.assign(new Error("intent already resolved"), { status: 409 }));
    render(<PendingAIIntents activeSessionId={null} />);

    fireEvent.click(await screen.findByRole("button", { name: "ai.intent_approve_execute" }));

    await waitFor(() => expect(vi.mocked(toast.error)).toHaveBeenCalledWith("intent already resolved"));
  });
});
