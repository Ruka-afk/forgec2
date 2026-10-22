import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import CircuitBreakerPage from "./CircuitBreakerPage";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn(), post: vi.fn(), postJson: vi.fn() },
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

vi.mock("sonner", () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

const getMock = vi.mocked(api.get);

const CONFIG = {
  failure_threshold: 3,
  cooldown_seconds: 300,
  half_open_max_reqs: 3,
  health_check_seconds: 60,
};

function mockApi(detail: unknown) {
  getMock.mockImplementation(async (path: unknown) => {
    const p = String(path);
    if (p === paths.circuitBreaker.detail) return detail;
    if (p === paths.circuitBreaker.config) return CONFIG;
    if (p === paths.circuitBreaker.events) return { events: [] };
    return {};
  });
}

describe("CircuitBreakerPage", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it("renders a listener whose fail_reasons arrived as null", async () => {
    // Go encodes an empty slice as JSON null; reading `.length` on it used to
    // throw inside render and replace the page with the error boundary.
    mockApi({
      listeners: [
        {
          target: "listener-1",
          scheme: "http",
          host: "127.0.0.1",
          port: 8001,
          status: "healthy",
          consecutive_fails: 0,
          last_probe: "",
          fail_reasons: null,
        },
      ],
    });

    render(
      <MemoryRouter>
        <CircuitBreakerPage />
      </MemoryRouter>,
    );

    expect(await screen.findByText("listener-1")).toBeTruthy();
    expect(screen.getByText("cb.monitored_listeners")).toBeTruthy();
    expect(screen.queryByText("Something went wrong")).toBeNull();
  });
});
