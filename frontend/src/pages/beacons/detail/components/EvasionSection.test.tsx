import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { EVASION_TECHNIQUES } from "@/lib/evasion-techniques";
import EvasionSection from "./EvasionSection";

vi.mock("@/lib/api", () => ({
  api: { post: vi.fn() },
}));

vi.mock("@/lib/i18n", () => {
  // Real interpolation for the one template this suite asserts on; every other
  // key resolves to itself. `t` must be referentially stable: the mutation's
  // callbacks carry it.
  const templates: Record<string, string> = {
    "agents.evasion_sent": "{technique} evasion technique sent",
  };
  const t = (key: string, params?: Record<string, string>) => {
    let s = templates[key] ?? key;
    if (params && typeof params === "object") {
      for (const [k, v] of Object.entries(params)) s = s.replace(`{${k}}`, String(v));
    }
    return s;
  };
  return { useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);
const TECH = EVASION_TECHNIQUES[0];

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  let reject: (reason?: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

async function chooseTechnique() {
  fireEvent.mouseDown(screen.getByRole("combobox"), { button: 0 });
  const option = await screen.findByRole("option", { name: TECH.labelKey });
  fireEvent.click(option);
  // Flush base-ui's post-focus timeout so its state update lands inside act.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

function executeButton() {
  return screen.getByRole("button", { name: /agents\.evasion_(execute|confirm|executing)/ });
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.post).mockResolvedValue({ success: true });
});

describe("EvasionSection", () => {
  it("arms first and only posts after the destructive confirm", async () => {
    render(<EvasionSection agentId="a1" online />);
    await chooseTechnique();

    fireEvent.click(executeButton());
    // First click only arms the destructive step: nothing has been sent.
    expect(api.post).not.toHaveBeenCalled();
    expect(screen.getByText("agents.evasion_confirm_msg")).toBeTruthy();

    await act(async () => {
      fireEvent.click(executeButton());
    });
    expect(api.post).toHaveBeenCalledWith(paths.agents.runEvasion("a1"), { technique: TECH.value });
  });

  it("names the technique in the success toast and clears the selection", async () => {
    render(<EvasionSection agentId="a1" online />);
    await chooseTechnique();
    fireEvent.click(executeButton());
    await act(async () => {
      fireEvent.click(executeButton());
    });

    await waitFor(() => expect(successToast).toHaveBeenCalledWith(`${TECH.labelKey} evasion technique sent`));
    // The selection reset also disarms the confirm step.
    await waitFor(() => expect((executeButton() as HTMLButtonElement).disabled).toBe(true));
  });

  it("surfaces the backend message when the execute is rejected", async () => {
    vi.mocked(api.post).mockRejectedValue({ status: 400, message: "EDR blocked the injection" });
    render(<EvasionSection agentId="a1" online />);
    await chooseTechnique();
    fireEvent.click(executeButton());
    await act(async () => {
      fireEvent.click(executeButton());
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("EDR blocked the injection"));
  });

  it("disables the button in flight and re-enables it after a failure", async () => {
    const pending = deferred<Record<string, unknown>>();
    vi.mocked(api.post).mockImplementation(() => pending.promise);
    render(<EvasionSection agentId="a1" online />);
    await chooseTechnique();
    fireEvent.click(executeButton());
    await act(async () => {
      fireEvent.click(executeButton());
    });

    await waitFor(() => expect((executeButton() as HTMLButtonElement).disabled).toBe(true));

    await act(async () => {
      pending.reject({ status: 500, message: "boom" });
    });
    await waitFor(() => expect((executeButton() as HTMLButtonElement).disabled).toBe(false));
  });

  it("clears the armed confirmation when the operator switches agents", async () => {
    const { rerender } = render(<EvasionSection agentId="a1" online />);
    await chooseTechnique();
    fireEvent.click(executeButton());
    expect(screen.getByText("agents.evasion_confirm_msg")).toBeTruthy();

    rerender(<EvasionSection agentId="a2" online />);

    // The armed technique must not stay aimed at the new implant.
    expect(screen.queryByText("agents.evasion_confirm_msg")).toBeNull();
    expect((executeButton() as HTMLButtonElement).disabled).toBe(true);
  });
});
