import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ReloadStatusCard from "./ReloadStatusCard";

/**
 * The reload-status probe used to swallow the failure reason and toast the
 * fixed card copy. The card body keeps that copy for an unread matrix; the
 * toast now carries the resolved reason so an operator can tell a dead
 * backend from a failed poll.
 */

const { getMock, toastError } = vi.hoisted(() => ({
  getMock: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: { get: (...args: unknown[]) => getMock(...args) },
  getRateLimitRetryAfter: () => 5,
}));

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), {
    success: vi.fn(),
    error: (...args: unknown[]) => toastError(...args),
    info: vi.fn(),
    warning: vi.fn(),
  }),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

describe("ReloadStatusCard", () => {
  beforeEach(() => {
    getMock.mockReset();
    toastError.mockReset();
  });

  it("surfaces the probe failure reason instead of the fixed copy", async () => {
    getMock.mockRejectedValue({ status: 500, message: "reload status exploded" });
    render(<ReloadStatusCard />);

    await waitFor(() => expect(toastError).toHaveBeenCalledWith("common.server_error"));
  });

  it("renders the hot/static matrix when the probe succeeds", async () => {
    getMock.mockResolvedValue({
      groups: [{ token: "agent.interval", mode: "hot", mechanism: "hook" }],
      last_reload: null,
    });
    render(<ReloadStatusCard />);

    await waitFor(() => expect(document.body.textContent).toContain("settings.reload_hot_count"));
    expect(toastError).not.toHaveBeenCalled();
  });
});
