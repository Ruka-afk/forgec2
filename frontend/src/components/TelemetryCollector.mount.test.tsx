import { act, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import TelemetryCollector from "./TelemetryCollector";
import { getTelemetryEntries } from "@/lib/telemetry";

describe("TelemetryCollector mounting", () => {
  afterEach(() => {
    // Collectors clean up after themselves; nothing global to reset.
  });

  it("records window errors and stays single-registered across login+app mounts", () => {
    const before = getTelemetryEntries().length;
    // Login route mount (pre-auth coverage).
    const login = render(<TelemetryCollector />);
    act(() => {
      window.dispatchEvent(new ErrorEvent("error", { message: "login boom" }));
    });
    expect(getTelemetryEntries().length).toBe(before + 1);

    // App mount while login still mounted (StrictMode-style overlap):
    // still exactly one registration, not two.
    const app = render(<TelemetryCollector />);
    act(() => {
      window.dispatchEvent(new ErrorEvent("error", { message: "app boom" }));
    });
    expect(getTelemetryEntries().length).toBe(before + 2);

    // Unmount order must not strand the survivor: login goes away first,
    // app stays protected.
    login.unmount();
    act(() => {
      window.dispatchEvent(new ErrorEvent("error", { message: " Survivor boom" }));
    });
    expect(getTelemetryEntries().length).toBe(before + 3);

    app.unmount();
    const settled = getTelemetryEntries().length;
    act(() => {
      window.dispatchEvent(new ErrorEvent("error", { message: "after unmount" }));
    });
    expect(getTelemetryEntries().length).toBe(settled);
  });
});
