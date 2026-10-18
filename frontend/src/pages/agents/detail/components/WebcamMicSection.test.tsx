import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import WebcamMicSection from "./WebcamMicSection";

vi.mock("@/lib/api", () => ({
  api: { postJson: vi.fn(), get: vi.fn(), post: vi.fn() },
  pollTask: vi.fn(),
  getRateLimitRetryAfter: vi.fn(() => 5),
}));

vi.mock("@/lib/i18n", () => {
  // `t` must be referentially stable: the mutation's callbacks carry it.
  const t = (key: string) => key;
  return { useI18n: () => ({ t }), getLocale: () => "en" };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);
const mockedPostJson = vi.mocked(api.postJson);
const mockedGet = vi.mocked(api.get);
const mockedPoll = vi.mocked(pollTask);

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  let reject: (reason?: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function webcamButton() {
  return screen.getByRole("button", { name: /agents\.media_webcam/ });
}

function micButton() {
  return screen.getByRole("button", { name: /agents\.media_mic/ });
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedPostJson.mockResolvedValue({ task_id: 5 });
  mockedPoll.mockResolvedValue({ status: "completed", id: 5, result: "QUJD" });
  mockedGet.mockResolvedValue({ result: "QUJD" });
});

describe("WebcamMicSection", () => {
  it("stores a webcam frame and toasts success", async () => {
    const { container } = render(<WebcamMicSection agentId="a1" online />);

    await act(async () => {
      fireEvent.click(webcamButton());
    });

    await waitFor(() => expect(successToast).toHaveBeenCalledWith("agents.media_captured"));
    expect(container.querySelector("img")).not.toBeNull();
    expect(mockedPostJson).toHaveBeenCalledWith(paths.toolkit.action("a1"), {
      action: "webcam",
      param: "",
      shell: "",
    });
  });

  it("surfaces the backend message when the capture is rejected", async () => {
    mockedPostJson.mockRejectedValue({ status: 400, message: "ffmpeg missing" });
    render(<WebcamMicSection agentId="a1" online />);

    await act(async () => {
      fireEvent.click(webcamButton());
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("ffmpeg missing"));
  });

  it("reports an empty capture without adding it to the gallery", async () => {
    mockedGet.mockResolvedValue({ result: "" });
    const { container } = render(<WebcamMicSection agentId="a1" online />);

    await act(async () => {
      fireEvent.click(webcamButton());
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("agents.media_empty"));
    expect(container.querySelector("img")).toBeNull();
    expect(successToast).not.toHaveBeenCalled();
  });

  it("blocks the other capture while one is in flight", async () => {
    const pending = deferred<Record<string, unknown>>();
    mockedPostJson.mockImplementation(() => pending.promise);
    render(<WebcamMicSection agentId="a1" online />);

    fireEvent.click(webcamButton());
    await waitFor(() => expect((micButton() as HTMLButtonElement).disabled).toBe(true));

    await act(async () => {
      pending.resolve({ task_id: 5 });
    });
    await waitFor(() => expect((micButton() as HTMLButtonElement).disabled).toBe(false));
  });

  it("clears the gallery when the operator switches agents", async () => {
    const { container, rerender } = render(<WebcamMicSection agentId="a1" online />);
    await act(async () => {
      fireEvent.click(webcamButton());
    });
    await waitFor(() => expect(container.querySelector("img")).not.toBeNull());

    rerender(<WebcamMicSection agentId="a2" online />);

    // Captures are per-agent evidence; the previous agent's frame must go.
    expect(container.querySelector("img")).toBeNull();
  });
});
