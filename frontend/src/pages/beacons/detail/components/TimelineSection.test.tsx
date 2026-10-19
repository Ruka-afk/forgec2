import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import TimelineSection from "./TimelineSection";

const getMock = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", () => ({
  api: { get: getMock },
  getRateLimitRetryAfter: vi.fn(() => null),
}));

vi.mock("@/lib/i18n", () => {
  const t = (key: string) => key;
  return { getLocale: () => "en", useI18n: () => ({ t }) };
});

describe("TimelineSection", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it("shows the unreadable banner when the timeline read fails", async () => {
    getMock.mockRejectedValue({ status: 500, message: "timeline exploded" });
    render(<TimelineSection agentId="a1" online />);

    // The banner replaces the reassuring "no history" line, which would read
    // as a fact about the agent rather than about the request.
    expect(await screen.findByText("agents.timeline_unreadable")).toBeTruthy();
  });

  it("renders fetched events on a successful read", async () => {
    getMock.mockResolvedValue({
      events: [{ time: new Date().toISOString(), kind: "task", title: "ran whoami" }],
    });
    render(<TimelineSection agentId="a1" online />);

    expect(await screen.findByText("ran whoami")).toBeTruthy();
  });
});
