import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { UpdateDialog, openUpdateDialog, OPEN_UPDATE_DIALOG } from "@/components/UpdateDialog";
import { api } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), postJson: vi.fn() },
}));

vi.mock("@/lib/wsContext", () => ({
  useWS: () => ({ subscribe: () => () => {} }),
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => {
    if (params && typeof params === "object") {
      let s: string = key;
      for (const [k, v] of Object.entries(params)) s = s.replace(`{${k}}`, String(v));
      return s;
    }
    return key;
  } }),
}));

function openDialog() {
  render(<UpdateDialog />);
  act(() => {
    openUpdateDialog({ latest: "v9.9.9", current: "v9.9.8" });
  });
}

describe("UpdateDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("opens on event and shows version comparison", async () => {
    openDialog();
    expect(await screen.findByText("update_dialog.title")).toBeTruthy();
    expect(screen.getByText("v9.9.9")).toBeTruthy();
  });

  it("starts hot update and shows progress", async () => {
    vi.mocked(api.postJson).mockResolvedValue({ success: true });
    openDialog();
    await screen.findByText("update_dialog.title");
    fireEvent.click(screen.getByText("update_dialog.update_now"));
    await waitFor(() => {
      expect(api.postJson).toHaveBeenCalledWith("/api/update-check/hot-update", {});
    });
    expect(await screen.findByText("update_dialog.downloading")).toBeTruthy();
  });

  it("shows error state when hot update fails", async () => {
    vi.mocked(api.postJson).mockRejectedValue(new Error("nope"));
    openDialog();
    await screen.findByText("update_dialog.title");
    fireEvent.click(screen.getByText("update_dialog.update_now"));
    expect(await screen.findByText("update_dialog.failed")).toBeTruthy();
  });

  it("ignores events without a version", () => {
    render(<UpdateDialog />);
    act(() => {
      window.dispatchEvent(new CustomEvent(OPEN_UPDATE_DIALOG, { detail: {} }));
    });
    expect(screen.queryByText("update_dialog.title")).toBeNull();
  });
});
