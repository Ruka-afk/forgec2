import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import BackupSection from "./BackupSection";

/**
 * A failed backup-list read must not render as "there are no backups", and the
 * failure must resolve through the shared rule once: the same text feeds the
 * inline unread warning and the toast, so the two surfaces cannot disagree.
 */

const { getMock, toastError, t } = vi.hoisted(() => ({
  getMock: vi.fn(),
  toastError: vi.fn(),
  t: (key: string, params?: Record<string, unknown>) =>
    params && typeof params.message === "string" ? `${key}·${params.message}` : key,
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

vi.mock("@/lib/i18n", () => ({
  getLocale: () => "en",
  useI18n: () => ({ t }),
}));

const bodyText = () => document.body.textContent ?? "";

describe("BackupSection load failure", () => {
  beforeEach(() => {
    getMock.mockReset();
    toastError.mockReset();
  });

  it("resolves the 5xx once for both the unread warning and the toast", async () => {
    getMock.mockRejectedValue({ status: 500, message: "backup store down" });
    render(<BackupSection />);

    await waitFor(() => expect(bodyText()).toContain("common.server_error"));
    expect(toastError).toHaveBeenCalledWith("common.server_error");
  });

  it("keeps the backup fallback on both surfaces when nothing carries a message", async () => {
    getMock.mockRejectedValue(new Error(""));
    render(<BackupSection />);

    await waitFor(() => expect(bodyText()).toContain("settings.toast.load_backups_failed"));
    expect(toastError).toHaveBeenCalledWith("settings.toast.load_backups_failed");
  });
});
