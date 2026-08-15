import { describe, expect, it } from "vitest";
import { dockCommandLabel, parseSleepArgs, parseSocksPort } from "./dock-commands";

describe("parseSleepArgs", () => {
  it("accepts interval and optional jitter", () => {
    expect(parseSleepArgs("60")).toEqual({ interval: 60, jitter: 0 });
    expect(parseSleepArgs("60,20")).toEqual({ interval: 60, jitter: 20 });
    expect(parseSleepArgs(" 45 10 ")).toEqual({ interval: 45, jitter: 10 });
    expect(parseSleepArgs("")).toBeNull();
    expect(parseSleepArgs("0")).toBeNull();
    expect(parseSleepArgs("60,101")).toBeNull();
  });
});

describe("parseSocksPort", () => {
  it("accepts a listen port", () => {
    expect(parseSocksPort("1080")).toBe(1080);
    expect(parseSocksPort("0")).toBeNull();
    expect(parseSocksPort("70000")).toBeNull();
  });
});

describe("dockCommandLabel", () => {
  it("maps kinds to implant task types", () => {
    expect(dockCommandLabel("ps")).toBe("ps");
    expect(dockCommandLabel("screenshot")).toBe("screenshot");
    expect(dockCommandLabel("sleep")).toBe("set_sleep");
  });
});
