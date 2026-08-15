import { describe, expect, it } from "vitest";
import {
  healthForListener,
  healthIndicatorStatus,
  indexListenerHealth,
  isProblemHealth,
  listenerHealthKey,
  translateHealthStatus,
  type ListenerHealth,
} from "./listener-health";

const row = (over: Partial<ListenerHealth> = {}): ListenerHealth => ({
  target: "3",
  status: "healthy",
  consecutive_fails: 0,
  last_probe: "2026-08-13T00:00:00Z",
  fail_reasons: [],
  ...over,
});

describe("listenerHealthKey", () => {
  it("stringifies numeric listener ids", () => {
    expect(listenerHealthKey(12)).toBe("12");
    expect(listenerHealthKey("12")).toBe("12");
  });
  it("treats empty as missing", () => {
    expect(listenerHealthKey("")).toBe("");
    expect(listenerHealthKey(undefined)).toBe("");
    expect(listenerHealthKey(null)).toBe("");
  });
});

describe("indexListenerHealth", () => {
  it("indexes by target and skips redirectors", () => {
    const map = indexListenerHealth([
      row({ target: "1", status: "healthy" }),
      row({ target: "redirector:9", status: "burned" }),
      row({ target: "2", status: "unstable" }),
    ]);
    expect(Object.keys(map).sort()).toEqual(["1", "2"]);
    expect(map["2"]?.status).toBe("unstable");
  });
});

describe("healthForListener", () => {
  it("matches list id to probe target", () => {
    const map = indexListenerHealth([row({ target: "7", status: "burned" })]);
    expect(healthForListener(map, 7)?.status).toBe("burned");
    expect(healthForListener(map, "7")?.status).toBe("burned");
    expect(healthForListener(map, 8)).toBeUndefined();
  });
});

describe("isProblemHealth", () => {
  it("flags unstable and burned only", () => {
    expect(isProblemHealth(row({ status: "healthy" }))).toBe(false);
    expect(isProblemHealth(row({ status: "unstable" }))).toBe(true);
    expect(isProblemHealth(row({ status: "burned" }))).toBe(true);
    expect(isProblemHealth(undefined)).toBe(false);
  });
});

describe("health labels", () => {
  it("maps probe status to indicator + i18n key", () => {
    const t = (key: string) => key;
    expect(healthIndicatorStatus("burned")).toBe("burned");
    expect(healthIndicatorStatus("closed")).toBe("unknown");
    expect(translateHealthStatus(t, "healthy")).toBe("listeners.health_healthy");
    expect(translateHealthStatus(t, undefined, false)).toBe("listeners.health_unmonitored");
  });
});
