import { describe, it, expect } from "vitest";
import { parseSleepInput } from "./useAgentBulkOps";

describe("parseSleepInput", () => {
  it("accepts in-range interval/jitter", () => {
    expect(parseSleepInput("30", "20")).toEqual({ interval: 30, jitter: 20 });
    expect(parseSleepInput("1", "0")).toEqual({ interval: 1, jitter: 0 });
    expect(parseSleepInput("86400", "100")).toEqual({ interval: 86400, jitter: 100 });
  });

  it("rejects out-of-range values", () => {
    expect(parseSleepInput("0", "20")).toBeNull();
    expect(parseSleepInput("86401", "20")).toBeNull();
    expect(parseSleepInput("30", "-1")).toBeNull();
    expect(parseSleepInput("30", "101")).toBeNull();
  });

  it("rejects non-numeric input", () => {
    expect(parseSleepInput("", "20")).toBeNull();
    expect(parseSleepInput("abc", "20")).toBeNull();
    expect(parseSleepInput("30", "abc")).toBeNull();
    // Number("") is 0: empty jitter coerces to 0, same as the legacy inline check.
    expect(parseSleepInput("30", "")).toEqual({ interval: 30, jitter: 0 });
  });
});
