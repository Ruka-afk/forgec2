import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { HostInfoCard } from "./HostInfoCard";

const apiMock = vi.hoisted(() => ({
  postFormData: vi.fn(),
  get: vi.fn(),
  pollTask: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: { postFormData: apiMock.postFormData, get: apiMock.get },
  pollTask: apiMock.pollTask,
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

async function renderAndAutoCollect() {
  vi.useFakeTimers();
  const view = render(<HostInfoCard agentId="a1" online />);
  // The card body (including the error line) lives in the collapsed section;
  // expand it like an operator would.
  const trigger = view.container.querySelector('[data-slot="collapsible-trigger"]');
  await act(async () => {
    fireEvent.click(trigger as Element);
  });
  // The card auto-collects shortly after mount when the agent is online.
  await act(async () => {
    await vi.advanceTimersByTimeAsync(900);
  });
}

describe("HostInfoCard", () => {
  beforeEach(() => {
    apiMock.postFormData.mockReset();
    apiMock.get.mockReset();
    apiMock.pollTask.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("maps a failed sweep to the shared server-error copy", async () => {
    apiMock.postFormData.mockRejectedValue({ status: 500, message: "hostinfo exploded" });
    await renderAndAutoCollect();

    expect(screen.getByText("common.server_error")).toBeTruthy();
  });

  it("keeps the backend text for an outcome-specific rejection", async () => {
    apiMock.postFormData.mockRejectedValue({ status: 400, message: "hostinfo denied" });
    await renderAndAutoCollect();

    expect(screen.getByText("hostinfo denied")).toBeTruthy();
  });
});
