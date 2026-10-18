import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { EntraIdentityPanel } from "./EntraIdentityPanel";

vi.mock("@/lib/api", () => ({
  api: { postJson: vi.fn() },
}));

vi.mock("@/lib/i18n", () => ({
  getLocale: () => "en",
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => {
      if (params && typeof params === "object") {
        let s: string = key;
        for (const [k, v] of Object.entries(params)) s = s.replace(`{${k}}`, String(v));
        return s;
      }
      return key;
    },
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const successToast = vi.mocked(toast.success);
const errorToast = vi.mocked(toast.error);

const ACTION_BUTTONS = [
  "phishing.entra_device_start",
  "phishing.entra_device_poll",
  "phishing.entra_consent_start",
  "phishing.entra_consent_exchange",
];

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  let reject: (reason?: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function button(name: string): HTMLButtonElement {
  return screen.getByRole("button", { name }) as HTMLButtonElement;
}

// The shared inputs have sibling labels (no htmlFor), so the fields are
// addressed by their textbox order: tenant, client id, scope, redirect URI.
function fillSharedFields() {
  const boxes = screen.getAllByRole("textbox") as HTMLInputElement[];
  fireEvent.change(boxes[1], { target: { value: "lab-client-id" } });
  fireEvent.change(boxes[3], { target: { value: "https://c2.lab/phishing/oauth/callback" } });
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.postJson).mockResolvedValue({});
});

describe("EntraIdentityPanel mutations", () => {
  it("surfaces the backend message when a device-code start is rejected", async () => {
    vi.mocked(api.postJson).mockRejectedValueOnce({ status: 400, message: "tenant not found" });
    render(<EntraIdentityPanel />);
    fillSharedFields();

    fireEvent.click(button("phishing.entra_device_start"));

    await waitFor(() => expect(api.postJson).toHaveBeenCalledWith(paths.identity.deviceCode, expect.anything()));
    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("tenant not found"));
  });

  it("keeps every action disabled while any request is still in flight", async () => {
    const deviceGate = deferred<Record<string, unknown>>();
    const consentGate = deferred<Record<string, unknown>>();
    vi.mocked(api.postJson).mockImplementation((path: string) => {
      if (path === paths.identity.deviceCode) return deviceGate.promise;
      if (path === paths.identity.consent) return consentGate.promise;
      return Promise.resolve({});
    });

    render(<EntraIdentityPanel />);
    fillSharedFields();

    // Two clicks in one commit window: both requests leave before either
    // pending state flushes (the old shared boolean allowed exactly this).
    act(() => {
      fireEvent.click(button("phishing.entra_device_start"));
      fireEvent.click(button("phishing.entra_consent_start"));
    });

    expect(vi.mocked(api.postJson)).toHaveBeenCalledTimes(2);
    for (const name of ACTION_BUTTONS) expect(button(name).disabled).toBe(true);

    // The first completion must not re-enable the buttons while the consent
    // start is still pending — the old shared boolean did exactly that.
    await act(async () => { deviceGate.resolve({ id: "d1", user_code: "ABCD" }); });
    expect(button("phishing.entra_consent_start").disabled).toBe(true);
    expect(button("phishing.entra_device_start").disabled).toBe(true);

    await act(async () => { consentGate.resolve({ id: "c1", authorize_url: "https://idp.lab/authorize" }); });
    expect(button("phishing.entra_device_start").disabled).toBe(false);
    expect(button("phishing.entra_consent_start").disabled).toBe(false);
    // Both follow-up actions become available from the resolved state.
    expect(button("phishing.entra_device_poll").disabled).toBe(false);
    expect(button("phishing.entra_consent_exchange").disabled).toBe(false);
  });

  it("renders the polled token JSON on the device flow", async () => {
    vi.mocked(api.postJson).mockImplementation((path: string) => {
      if (path === paths.identity.deviceCode) {
        return Promise.resolve({ id: "d1", user_code: "ABCD", verification_uri: "https://microsoft.com/devicelogin" });
      }
      if (path === paths.identity.deviceCodePoll("d1")) return Promise.resolve({ access_token: "tok-123" });
      return Promise.resolve({});
    });

    render(<EntraIdentityPanel />);
    fillSharedFields();

    fireEvent.click(button("phishing.entra_device_start"));
    await waitFor(() => expect(successToast).toHaveBeenCalledWith("phishing.entra_device_started"));
    expect(screen.getByText(/devicelogin/)).toBeTruthy();

    fireEvent.click(button("phishing.entra_device_poll"));
    await waitFor(() => expect(api.postJson).toHaveBeenCalledWith(paths.identity.deviceCodePoll("d1"), {}));
    await waitFor(() => expect(screen.getByText("phishing.entra_token")).toBeTruthy());
    expect((screen.getByDisplayValue(/tok-123/) as HTMLTextAreaElement).value).toContain("tok-123");
  });
});
