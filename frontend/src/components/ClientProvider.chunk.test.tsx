import { afterEach, describe, expect, it } from "vitest";
import { chunkReloadDecision } from "./ClientProvider";

describe("chunkReloadDecision", () => {
  afterEach(() => {
    sessionStorage.clear();
  });

  it("reloads a first chunk error outside the throttle window", () => {
    expect(chunkReloadDecision(1_000_000)).toBe("reload");
  });

  it("throttles bursts within 15s", () => {
    sessionStorage.setItem("chunkErrorReloadAt", String(1_000_000));
    expect(chunkReloadDecision(1_005_000)).toBe("throttled");
    expect(chunkReloadDecision(1_020_000)).toBe("reload");
  });

  it("stops after two auto-reloads so a broken build cannot loop tabs", () => {
    sessionStorage.setItem("chunkErrorReloadAt", "0");
    sessionStorage.setItem("chunkErrorReloadCount", "2");
    expect(chunkReloadDecision(1_000_000)).toBe("exhausted");
  });

  it("reloads when storage is unavailable (old behavior)", () => {
    const getItem = sessionStorage.getItem;
    sessionStorage.getItem = () => { throw new Error("denied"); };
    try {
      expect(chunkReloadDecision()).toBe("reload");
    } finally {
      sessionStorage.getItem = getItem;
    }
  });
});
