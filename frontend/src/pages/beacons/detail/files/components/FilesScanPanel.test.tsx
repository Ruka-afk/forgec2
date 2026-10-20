import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { api, pollTask } from "@/lib/api";
import FilesScanPanel from "./FilesScanPanel";

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn() },
  pollTask: vi.fn(),
  getRateLimitRetryAfter: vi.fn(() => 0),
}));

const t = (key: string) => key;

function scanResult(overrides: Record<string, unknown> = {}) {
  return JSON.stringify({
    root: "C:\\Users\\alice",
    rules: ["email", "phone", "bank_card"],
    files_scanned: 12,
    files_skipped: 5,
    skipped_binary: 4,
    skipped_large: 1,
    bytes_scanned: 4096,
    findings: [
      {
        path: "C:\\Users\\alice\\notes.txt",
        name: "notes.txt",
        rule: "email",
        value: "alice@example.com",
        line: 4,
        occurrences: 2,
      },
    ],
    counts: { email: 1 },
    errors: 0,
    truncated: false,
    elapsed_ms: 32,
    ...overrides,
  });
}

beforeEach(() => {
  vi.mocked(api.post).mockReset();
  vi.mocked(pollTask).mockReset();
  vi.mocked(api.post).mockResolvedValue({ success: true, task_id: 9 });
});

describe("FilesScanPanel", () => {
  it("queues the default detectors and renders raw findings", async () => {
    vi.mocked(pollTask).mockResolvedValue({ status: "completed", id: 9, result: scanResult() });
    const onReveal = vi.fn();
    render(<FilesScanPanel t={t} agentId="a1" rootPath={"C:\\Users\\alice"} onReveal={onReveal} />);

    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes).toHaveLength(9);
    expect(checkboxes[0].getAttribute("aria-checked")).toBe("true");
    expect(checkboxes[1].getAttribute("aria-checked")).toBe("true");
    expect(checkboxes[2].getAttribute("aria-checked")).toBe("true");
    expect(checkboxes[3].getAttribute("aria-checked")).toBe("false");

    fireEvent.click(screen.getByRole("button", { name: "agents.files_scan_start" }));

    await waitFor(() => expect(screen.getByText("alice@example.com")).toBeTruthy());
    expect(api.post).toHaveBeenCalledWith(
      "/agents/a1/files/scan",
      expect.objectContaining({
        path: "C:\\Users\\alice",
        pattern: "*",
        rules: "email,phone,bank_card",
        include_hidden: "0",
        max_depth: "8",
        max_files: "200",
        max_file_bytes: String(1024 * 1024),
        max_total_bytes: String(32 * 1024 * 1024),
        max_findings: "500",
        timeout_seconds: "30",
      }),
    );
    expect(pollTask).toHaveBeenCalledWith("a1", 9, expect.objectContaining({ timeoutMs: 120_000 }));
    expect(screen.getAllByText("agents.files_scan_rule_email").length).toBeGreaterThan(1);
    expect(screen.getByText("agents.files_scan_found")).toBeTruthy();
    expect(screen.getByText("agents.files_scan_summary")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "agents.files_scan_reveal" }));
    expect(onReveal).toHaveBeenCalledWith("C:\\Users\\alice\\notes.txt", "notes.txt");
  });

  it("requires at least one detector and sends opt-in rules explicitly", async () => {
    vi.mocked(pollTask).mockResolvedValue({
      status: "completed",
      id: 9,
      result: scanResult({ findings: [] }),
    });
    render(<FilesScanPanel t={t} agentId="a1" rootPath="/home/alice" onReveal={vi.fn()} />);

    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[0]);
    fireEvent.click(checkboxes[1]);
    fireEvent.click(checkboxes[2]);
    expect(screen.getByRole("alert").textContent).toContain("agents.files_scan_pick_rule");
    expect((screen.getByRole("button", { name: "agents.files_scan_start" }) as HTMLButtonElement).disabled).toBe(true);

    fireEvent.click(checkboxes[3]);
    expect((screen.getByRole("button", { name: "agents.files_scan_start" }) as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "agents.files_scan_start" }));

    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith(
        "/agents/a1/files/scan",
        expect.objectContaining({ rules: "ssn" }),
      ),
    );
  });

  it("surfaces truncation and unreadable paths", async () => {
    vi.mocked(pollTask).mockResolvedValue({
      status: "completed",
      id: 9,
      result: scanResult({ truncated: true, truncated_reason: "max_total_bytes", errors: 2 }),
    });
    render(<FilesScanPanel t={t} agentId="a1" rootPath="/home/alice" onReveal={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "agents.files_scan_start" }));

    await waitFor(() => expect(screen.getByText(/agents\.files_scan_truncated/)).toBeTruthy());
    expect(screen.getByText(/agents\.files_scan_trunc_max_total_bytes/)).toBeTruthy();
    expect(screen.getByText("agents.files_scan_errors")).toBeTruthy();
  });

  it("rejects an unreadable result without showing an empty scan", async () => {
    vi.mocked(pollTask).mockResolvedValue({ status: "completed", id: 9, result: `{"findings":null}` });
    render(<FilesScanPanel t={t} agentId="a1" rootPath="/home/alice" onReveal={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "agents.files_scan_start" }));

    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByText("agents.files_scan_bad_result")).toBeTruthy();
    expect(screen.queryByText("agents.files_scan_empty")).toBeNull();
  });

  it("shows task failures and restores the start button", async () => {
    vi.mocked(pollTask).mockResolvedValue({ status: "failed", id: 9, error: "scan exploded" });
    render(<FilesScanPanel t={t} agentId="a1" rootPath="/home/alice" onReveal={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "agents.files_scan_start" }));

    await waitFor(() => expect(screen.getByText("scan exploded")).toBeTruthy());
    expect(screen.getByRole("button", { name: "agents.files_scan_start" })).toBeTruthy();
  });
});
