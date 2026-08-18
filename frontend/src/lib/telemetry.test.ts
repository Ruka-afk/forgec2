import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  getTelemetryEntries,
  clearTelemetry,
  recordVital,
  recordClientError,
  subscribeTelemetry,
} from "./telemetry";

describe("telemetry ring buffer", () => {
  beforeEach(() => {
    clearTelemetry();
  });

  it("records vitals and errors", () => {
    recordVital("LCP", 2500);
    recordClientError("window.error", "boom");
    const entries = getTelemetryEntries();
    expect(entries).toHaveLength(2);
    expect(entries[0]).toMatchObject({ kind: "vital", name: "LCP", value: 2500 });
    expect(entries[1]).toMatchObject({ kind: "error", source: "window.error", message: "boom" });
  });

  it("rejects non-finite vital values", () => {
    recordVital("CLS", NaN);
    recordVital("CLS", Infinity);
    expect(getTelemetryEntries()).toHaveLength(0);
  });

  it("rejects empty error messages", () => {
    recordClientError("window.error", "");
    recordClientError("window.error", null as unknown as string);
    expect(getTelemetryEntries()).toHaveLength(0);
  });

  it("truncates long error messages", () => {
    recordClientError("x", "a".repeat(500));
    const entry = getTelemetryEntries()[0];
    expect(entry.kind === "error" && entry.message.length).toBe(300);
  });

  it("caps the buffer at 100 entries, dropping oldest", () => {
    for (let i = 0; i < 120; i++) recordVital("TTFB", i);
    const entries = getTelemetryEntries();
    expect(entries).toHaveLength(100);
    expect(entries[0]).toMatchObject({ value: 20 });
    expect(entries[99]).toMatchObject({ value: 119 });
  });

  it("notifies subscribers and clear notifies too", () => {
    const fn = vi.fn();
    const unsubscribe = subscribeTelemetry(fn);
    recordVital("FCP", 100);
    expect(fn).toHaveBeenCalledTimes(1);
    clearTelemetry();
    expect(fn).toHaveBeenCalledTimes(2);
    expect(getTelemetryEntries()).toHaveLength(0);
    unsubscribe();
    recordVital("FCP", 200);
    expect(fn).toHaveBeenCalledTimes(2);
  });
});