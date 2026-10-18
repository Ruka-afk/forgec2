import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import InjectSection from "./InjectSection";

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn() },
}));

vi.mock("@/lib/i18n", () => {
  // `t` must be referentially stable: the mutation's callbacks carry it.
  const t = (key: string) => key;
  return { useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  let reject: (reason?: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/** Base-ui select: mousedown opens the popup, click on the option selects. */
async function chooseTechnique(value: string, label: string) {
  fireEvent.mouseDown(screen.getByRole("combobox"), { button: 0 });
  const option = await screen.findByRole("option", { name: label });
  fireEvent.click(option);
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

async function stagePayload(container: HTMLElement) {
  fireEvent.change(screen.getByRole("spinbutton"), { target: { value: "4242" } });
  await chooseTechnique("createremotethread", "CreateRemoteThread");
  const fileInput = container.querySelector("input[type=file]") as HTMLInputElement;
  const file = new File([new Uint8Array([0x41, 0x42])], "sc.bin");
  fireEvent.change(fileInput, { target: { files: [file] } });
  await waitFor(() => expect(screen.getByText(/sc\.bin/)).toBeTruthy());
}

function injectButton() {
  return screen.getByRole("button", { name: /agents\.(inject_execute|inject_sending|evasion_confirm)/ });
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.post).mockResolvedValue({ success: true });
});

describe("InjectSection", () => {
  it("arms first, posts the staged payload on confirm and clears the form", async () => {
    const { container } = render(<InjectSection agentId="a1" online />);
    await stagePayload(container);

    fireEvent.click(injectButton());
    // First click only arms the destructive step: nothing has been sent.
    expect(api.post).not.toHaveBeenCalled();
    expect(screen.getByText("agents.inject_confirm_msg")).toBeTruthy();

    await act(async () => {
      fireEvent.click(injectButton());
    });
    expect(api.post).toHaveBeenCalledWith(paths.agents.inject("a1"), {
      pid: "4242",
      tech: "createremotethread",
      shellcode: "QUI=",
    });
    await waitFor(() => expect(successToast).toHaveBeenCalledWith("agents.inject_queued"));
    // The form resets so a double-click cannot re-fire the same payload.
    expect(screen.getByText("agents.inject_no_file")).toBeTruthy();
    expect((screen.getByRole("spinbutton") as HTMLInputElement).value).toBe("");
  });

  it("surfaces the backend message when the injection is rejected", async () => {
    vi.mocked(api.post).mockRejectedValue({ status: 400, message: "injection blocked by EDR" });
    const { container } = render(<InjectSection agentId="a1" online />);
    await stagePayload(container);

    fireEvent.click(injectButton());
    await act(async () => {
      fireEvent.click(injectButton());
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("injection blocked by EDR"));
  });

  it("keeps the button disabled while in flight and re-enables it after a failure", async () => {
    const pending = deferred<Record<string, unknown>>();
    vi.mocked(api.post).mockImplementation(() => pending.promise);
    const { container } = render(<InjectSection agentId="a1" online />);
    await stagePayload(container);

    fireEvent.click(injectButton());
    await act(async () => {
      fireEvent.click(injectButton());
    });
    await waitFor(() => expect((injectButton() as HTMLButtonElement).disabled).toBe(true));

    await act(async () => {
      pending.reject({ status: 500, message: "boom" });
    });
    await waitFor(() => expect((injectButton() as HTMLButtonElement).disabled).toBe(false));
  });

  it("clears the staged payload when the operator switches agents", async () => {
    const { container, rerender } = render(<InjectSection agentId="a1" online />);
    await stagePayload(container);
    fireEvent.click(injectButton());
    expect(screen.getByText("agents.inject_confirm_msg")).toBeTruthy();

    rerender(<InjectSection agentId="a2" online />);

    // Stale shellcode and confirmation must not stay aimed at the new implant.
    expect(screen.queryByText("agents.inject_confirm_msg")).toBeNull();
    expect(screen.getByText("agents.inject_no_file")).toBeTruthy();
    expect((screen.getByRole("spinbutton") as HTMLInputElement).value).toBe("");
  });

  it("rejects a non-positive pid without calling the API", async () => {
    const { container } = render(<InjectSection agentId="a1" online />);
    await stagePayload(container);
    fireEvent.change(screen.getByRole("spinbutton"), { target: { value: "0" } });

    fireEvent.click(injectButton());
    fireEvent.click(injectButton());

    expect(api.post).not.toHaveBeenCalled();
  });
});
