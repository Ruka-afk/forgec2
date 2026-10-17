import { describe, it, expect } from "vitest";
import { osColor, phaseColor } from "./chart-palette";

describe("phaseColor", () => {
  it("maps known MITRE phases to the ordered chart tokens", () => {
    expect(phaseColor("Reconnaissance")).toBe("var(--chart-1)");
    expect(phaseColor("Resource Development")).toBe("var(--chart-2)");
    expect(phaseColor("Execution")).toBe("var(--chart-4)");
    expect(phaseColor("Defense Evasion")).toBe("var(--chart-1)");
    expect(phaseColor("Impact")).toBe("var(--chart-2)");
  });

  it("falls back to a deterministic hash token for unknown phases", () => {
    const a = phaseColor("Custom Tactic");
    const b = phaseColor("Custom Tactic");

    expect(a).toBe(b);
    expect(a).toMatch(/^var\(--chart-[1-6]\)$/);
  });
});

describe("osColor", () => {
  it("maps known OS names to fixed tokens with a neutral default", () => {
    expect(osColor("Windows")).toBe("var(--chart-2)");
    expect(osColor("Linux")).toBe("var(--chart-4)");
    expect(osColor("macOS")).toBe("var(--chart-6)");
    expect(osColor("darwin")).toBe("var(--chart-6)");
    expect(osColor("FreeBSD")).toBe("var(--chart-3)");
  });
});
