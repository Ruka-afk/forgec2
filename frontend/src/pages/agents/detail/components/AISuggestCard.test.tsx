import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { ApiError } from "@/lib/api";
import { AISuggestCard } from "./AISuggestCard";

const apiMock = vi.hoisted(() => ({ postJson: vi.fn() }));

vi.mock("@/lib/api", () => ({
  api: { postJson: apiMock.postJson },
  ApiError: class ApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.name = "ApiError";
      this.status = status;
    }
  },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  // `t` must be referentially stable: loadConfig carries it in its deps, and a
  // fresh function per render would re-run the load effect forever.
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() },
}));

vi.mock("@/lib/hooks/useAIStatus", () => ({
  useAIStatus: () => ({ status: { known: true, enabled: true } }),
}));

const errorToast = vi.mocked(toast.error);
const warningToast = vi.mocked(toast.warning);

const SUGGESTION = {
  action: "Check privileges",
  reason: "the token is stale",
  risk: "low" as const,
  command_hint: "whoami",
};

function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

function loadButton() {
  return screen.getByRole("button", { name: "ai.suggest_load" });
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("AISuggestCard", () => {
  it("maps a rejected suggestion load to the normalized message and re-enables the button", async () => {
    apiMock.postJson.mockRejectedValue({ status: 500, message: "ai backend down" });
    render(<AISuggestCard agentId="a1" online />);

    await act(async () => {
      fireEvent.click(loadButton());
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("common.server_error"));
    await waitFor(() => expect((loadButton() as HTMLButtonElement).disabled).toBe(false));
  });

  it("surfaces the backend text for a 4xx suggestion load", async () => {
    apiMock.postJson.mockRejectedValue({ status: 400, message: "model not configured" });
    render(<AISuggestCard agentId="a1" online />);

    await act(async () => {
      fireEvent.click(loadButton());
    });

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("model not configured"));
  });

  it("keeps the 409 approval notice a warning and routes other run failures through the normalizer", async () => {
    apiMock.postJson.mockResolvedValueOnce({ suggestions: [SUGGESTION] });
    render(<AISuggestCard agentId="a1" online />);
    await act(async () => {
      fireEvent.click(loadButton());
    });
    const run = await screen.findByRole("button", { name: "ai.suggest_run" });

    apiMock.postJson.mockRejectedValueOnce(new ApiError("destructive command needs approval", 409));
    await act(async () => {
      fireEvent.click(run);
    });
    await waitFor(() => expect(warningToast).toHaveBeenCalledWith("destructive command needs approval"));
    expect(errorToast).not.toHaveBeenCalled();
    await waitFor(() => expect((run as HTMLButtonElement).disabled).toBe(false));

    apiMock.postJson.mockRejectedValueOnce({ status: 400, message: "agent is offline" });
    await act(async () => {
      fireEvent.click(run);
    });
    await waitFor(() => expect(errorToast).toHaveBeenCalledWith("agent is offline"));
  });

  it("clears suggestions and drops an in-flight load when the agent switches", async () => {
    const pending = deferred<{ suggestions: typeof SUGGESTION[] }>();
    apiMock.postJson.mockImplementation(() => pending.promise);
    const { rerender } = render(<AISuggestCard agentId="a1" online />);
    await act(async () => {
      fireEvent.click(loadButton());
    });

    rerender(<AISuggestCard agentId="a2" online />);
    // The old load resolves after the switch: its suggestions must not land.
    await act(async () => {
      pending.resolve({ suggestions: [SUGGESTION] });
    });

    expect(screen.queryByRole("button", { name: "ai.suggest_run" })).toBeNull();
  });
});
