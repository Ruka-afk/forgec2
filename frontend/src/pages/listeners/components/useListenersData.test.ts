import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useListenersData } from "./useListenersData";
import { emptyCreateForm, emptyEditForm } from "./types";

const { getMock, postJsonMock, putJsonMock, toastError, tMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postJsonMock: vi.fn(),
  putJsonMock: vi.fn(),
  toastError: vi.fn(),
  tMock: (key: string) => key,
}));

vi.mock("@/lib/api", () => ({
  api: {
    get: (...args: unknown[]) => getMock(...args),
    postJson: (...args: unknown[]) => postJsonMock(...args),
    putJson: (...args: unknown[]) => putJsonMock(...args),
    post: vi.fn(),
    del: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: { error: (...a: unknown[]) => toastError(...a), success: vi.fn() },
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: tMock }),
}));

vi.mock("@/lib/wsContext", () => ({
  useWS: () => ({ subscribe: () => () => {} }),
}));

vi.mock("@/lib/agents", () => ({
  fetchAllAgents: async () => [],
}));

async function mountHook() {
  getMock.mockResolvedValue({ data: [] });
  const { result } = renderHook(() => useListenersData());
  await waitFor(() => expect(getMock).toHaveBeenCalled());
  return result;
}

function lastBody(mock: typeof postJsonMock | typeof putJsonMock) {
  const call = mock.mock.calls[mock.mock.calls.length - 1];
  return call[1] as Record<string, unknown>;
}

describe("useListenersData TLS posture payloads", () => {
  beforeEach(() => {
    getMock.mockReset();
    postJsonMock.mockReset();
    putJsonMock.mockReset();
    toastError.mockReset();
  });

  it("create sends the three TLS fields, with the CA path trimmed, for https/tls listeners", async () => {
    postJsonMock.mockResolvedValueOnce({ success: true });
    const result = await mountHook();

    await act(async () => {
      await result.current.createListener({
        ...emptyCreateForm(),
        name: "prod-mtls",
        type: "https",
        protocol: "https",
        host: "0.0.0.0",
        port: "8443",
        client_ca_file: "  /etc/forgec2/client-ca.pem  ",
        require_client_cert: true,
        min_tls_version: "1.3",
      });
    });

    expect(postJsonMock).toHaveBeenCalledWith(
      "/api/listeners",
      expect.objectContaining({
        name: "prod-mtls",
        scheme: "https",
        port: 8443,
        client_ca_file: "/etc/forgec2/client-ca.pem",
        require_client_cert: true,
        min_tls_version: "1.3",
      }),
    );
  });

  it("create omits the TLS fields for a plain scheme instead of sending knobs the server refuses", async () => {
    postJsonMock.mockResolvedValueOnce({ success: true });
    const result = await mountHook();

    await act(async () => {
      await result.current.createListener({
        ...emptyCreateForm(),
        name: "lab",
        type: "http",
        protocol: "http",
        host: "0.0.0.0",
        port: "8080",
        // Stale values left over from a scheme switch must not leak into a
        // create for a listener that has no TLS posture at all.
        client_ca_file: "/stale.pem",
        require_client_cert: true,
        min_tls_version: "1.3",
      });
    });

    const body = lastBody(postJsonMock);
    expect(body.scheme).toBe("http");
    expect(body).not.toHaveProperty("client_ca_file");
    expect(body).not.toHaveProperty("require_client_cert");
    expect(body).not.toHaveProperty("min_tls_version");
  });

  it("update keeps the posture for an https listener", async () => {
    putJsonMock.mockResolvedValueOnce({ success: true });
    const result = await mountHook();

    await act(async () => {
      await result.current.updateListener("7", {
        ...emptyEditForm(),
        name: "prod-mtls",
        type: "https",
        protocol: "https",
        host: "0.0.0.0",
        port: "8443",
        client_ca_file: "/etc/forgec2/client-ca.pem",
        require_client_cert: true,
        min_tls_version: "1.2",
      });
    });

    expect(putJsonMock).toHaveBeenCalledWith(
      "/api/listeners/7",
      expect.objectContaining({
        client_ca_file: "/etc/forgec2/client-ca.pem",
        require_client_cert: true,
        min_tls_version: "1.2",
      }),
    );
  });

  it("update clears the posture explicitly when the scheme no longer terminates TLS", async () => {
    putJsonMock.mockResolvedValueOnce({ success: true });
    const result = await mountHook();

    await act(async () => {
      await result.current.updateListener("9", {
        ...emptyEditForm(),
        name: "switched-to-tcp",
        type: "tcp",
        protocol: "tcp",
        host: "0.0.0.0",
        port: "9001",
        // Left over from the https days: the server refuses TLS knobs on a
        // non-TLS scheme, so the update has to clear them, not forward them.
        client_ca_file: "/etc/forgec2/client-ca.pem",
        require_client_cert: true,
        min_tls_version: "1.3",
      });
    });

    expect(putJsonMock).toHaveBeenCalledWith(
      "/api/listeners/9",
      expect.objectContaining({
        client_ca_file: "",
        require_client_cert: false,
        min_tls_version: "",
      }),
    );
  });

  it("create surfaces the server's validation message instead of a generic toast", async () => {
    postJsonMock.mockRejectedValueOnce(
      new Error("require_client_cert needs a client CA (set client_ca_file or server.client_ca_file)"),
    );
    const result = await mountHook();

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.createListener({
        ...emptyCreateForm(),
        name: "prod-mtls",
        type: "https",
        protocol: "https",
        host: "0.0.0.0",
        port: "8443",
        require_client_cert: true,
      });
    });

    expect(ok).toBe(false);
    expect(toastError).toHaveBeenCalledWith(
      "require_client_cert needs a client CA (set client_ca_file or server.client_ca_file)",
    );
  });

  it("create refuses a form without name/host/port before calling the API", async () => {
    const result = await mountHook();

    let ok: boolean | undefined;
    await act(async () => {
      ok = await result.current.createListener({ ...emptyCreateForm() });
    });

    expect(ok).toBe(false);
    expect(postJsonMock).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledWith("listeners.toast_name_host_port_required");
  });
});
