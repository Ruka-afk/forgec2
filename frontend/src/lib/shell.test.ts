import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("./api", () => ({ api: { get: mocks.get } }));

import { fetchAgentBeaconTiming, loadCommandHistory, saveCommandHistory } from "./shell";
import { SHELL_HISTORY_KEY } from "./shell-ui";

describe("fetchAgentBeaconTiming", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    localStorage.clear();
  });

  it("reads interval and jitter from the agent payload", async () => {
    mocks.get.mockResolvedValue({ agent: { current_interval: 30, current_jitter: 5 } });
    await expect(fetchAgentBeaconTiming("a1")).resolves.toEqual({ interval: 30, jitter: 5 });
    expect(mocks.get).toHaveBeenCalledWith("/toolkit/agents/a1/info?format=json");
  });

  it("coerces string values and defaults the missing ones", async () => {
    mocks.get.mockResolvedValue({ agent: { current_interval: "45" } });
    await expect(fetchAgentBeaconTiming("a1")).resolves.toEqual({ interval: 45, jitter: 20 });

    mocks.get.mockResolvedValue({ agent: { current_interval: null, current_jitter: null } });
    await expect(fetchAgentBeaconTiming("a1")).resolves.toEqual({ interval: 10, jitter: 20 });

    mocks.get.mockResolvedValue({});
    await expect(fetchAgentBeaconTiming("a1")).resolves.toEqual({ interval: 10, jitter: 20 });
  });

  it("falls back to the defaults when the request fails", async () => {
    mocks.get.mockRejectedValue(new Error("offline"));
    await expect(fetchAgentBeaconTiming("a1")).resolves.toEqual({ interval: 10, jitter: 20 });
  });
});

describe("command history", () => {
  beforeEach(() => localStorage.clear());

  it("returns an empty history when nothing is stored", () => {
    expect(loadCommandHistory()).toEqual([]);
  });

  it("returns an empty history for corrupt JSON", () => {
    localStorage.setItem(SHELL_HISTORY_KEY, "{not json");
    expect(loadCommandHistory()).toEqual([]);
  });

  it("dedupes on save and moves the command to the front", () => {
    localStorage.setItem(SHELL_HISTORY_KEY, JSON.stringify(["a", "b", "c"]));
    saveCommandHistory("b");
    expect(loadCommandHistory()).toEqual(["b", "a", "c"]);
  });

  it("caps the stored history at 100 entries", () => {
    const seed = Array.from({ length: 100 }, (_, i) => `cmd-${i}`);
    localStorage.setItem(SHELL_HISTORY_KEY, JSON.stringify(seed));
    saveCommandHistory("fresh");
    const hist = loadCommandHistory();
    expect(hist).toHaveLength(100);
    expect(hist[0]).toBe("fresh");
    expect(hist).not.toContain("cmd-99");
  });

  it("never throws when storage is blocked", () => {
    const getSpy = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(loadCommandHistory()).toEqual([]);
    getSpy.mockRestore();

    const setSpy = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("quota exceeded");
    });
    expect(() => saveCommandHistory("x")).not.toThrow();
    setSpy.mockRestore();
  });
});
