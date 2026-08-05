import { describe, it, expect } from "vitest";
import { formatBytes, formatUptime } from "./types";

describe("pivoting format helpers", () => {
  it("formatBytes", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1024)).toMatch(/KB/);
  });
  it("formatUptime", () => {
    expect(formatUptime(0)).toBe("-");
    expect(formatUptime(65)).toBe("1m 5s");
    expect(formatUptime(3661)).toBe("1h 1m 1s");
  });
});
