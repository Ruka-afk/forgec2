import { describe, it, expect, vi, afterEach } from "vitest";
import { getWSURL, sendWSMessage, subscribeAgent, unsubscribeAgent } from "./wsContext";

describe("getWSURL", () => {
  afterEach(() => {
    delete process.env.NEXT_PUBLIC_WS_URL;
    delete process.env.NEXT_PUBLIC_GO_BACKEND_PORT;
  });

  it("uses NEXT_PUBLIC_WS_URL override as-is", () => {
    process.env.NEXT_PUBLIC_WS_URL = "wss://ws.example.com";
    expect(getWSURL()).toBe("wss://ws.example.com/ws");
    expect(getWSURL("/custom")).toBe("wss://ws.example.com/custom");
  });

  it("builds ws:// from window.location when no env override", () => {
    const loc = { protocol: "http:", hostname: "c2.internal", port: "8443" };
    Object.defineProperty(window, "location", { configurable: true, value: loc, writable: true });
    delete process.env.NEXT_PUBLIC_WS_URL;
    delete process.env.NEXT_PUBLIC_GO_BACKEND_PORT;
    expect(getWSURL()).toBe("ws://c2.internal:8443/ws");
  });

  it("uses wss:// for https pages", () => {
    const loc = { protocol: "https:", hostname: "c2.example", port: "443" };
    Object.defineProperty(window, "location", { configurable: true, value: loc, writable: true });
    delete process.env.NEXT_PUBLIC_WS_URL;
    delete process.env.NEXT_PUBLIC_GO_BACKEND_PORT;
    expect(getWSURL()).toBe("wss://c2.example:443/ws");
  });
});

describe("WS control helpers without an open socket", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("sendWSMessage is a safe no-op when no socket is open", () => {
    expect(() => sendWSMessage({ type: "anything" })).not.toThrow();
  });

  it("subscribeAgent/unsubscribeAgent do not throw without a socket", () => {
    expect(() => subscribeAgent("agent-1")).not.toThrow();
    expect(() => unsubscribeAgent("agent-1")).not.toThrow();
  });
});