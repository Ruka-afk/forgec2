import { describe, expect, it } from "vitest";
import {
  canGenerateFromListener,
  canGeneratePayload,
  composeC2URL,
  listenerTransportCompatible,
  schemeForTransport,
} from "./generate-gate";

describe("canGenerateFromListener", () => {
  it("disables generate until a listener id is present", () => {
    expect(canGenerateFromListener("")).toBe(false);
    expect(canGenerateFromListener("   ")).toBe(false);
    expect(canGenerateFromListener(null)).toBe(false);
    expect(canGenerateFromListener(undefined)).toBe(false);
    expect(canGenerateFromListener("12")).toBe(true);
  });
});

describe("schemeForTransport", () => {
  it("keeps http listener scheme for http transport", () => {
    expect(schemeForTransport("http", "http")).toBe("http");
    expect(schemeForTransport("https", "http")).toBe("https");
  });
  it("uses tcp scheme when transport is tcp", () => {
    expect(schemeForTransport("http", "tcp")).toBe("tcp");
    expect(schemeForTransport("tls", "tcp")).toBe("tls");
  });
  it("uses transport scheme for udp/quic", () => {
    expect(schemeForTransport("http", "udp")).toBe("udp");
    expect(schemeForTransport("http", "quic")).toBe("quic");
  });
});

describe("composeC2URL", () => {
  it("builds scheme://host:port and appends failover", () => {
    expect(composeC2URL({ scheme: "http", host: "c2.lab", port: 8443 })).toBe("http://c2.lab:8443");
    expect(composeC2URL({ scheme: "tcp", host: "c2.lab", port: "4444", failover: "tcp://b:9" }))
      .toBe("tcp://c2.lab:4444,tcp://b:9");
  });
});

describe("listenerTransportCompatible", () => {
  it("allows matching families", () => {
    expect(listenerTransportCompatible("http", "http")).toBe(true);
    expect(listenerTransportCompatible("https", "wss")).toBe(true);
    expect(listenerTransportCompatible("tcp", "tcp")).toBe(true);
  });
  it("rejects HTTP listener + TCP transport without a tcp failover URL", () => {
    expect(listenerTransportCompatible("http", "tcp")).toBe(false);
    expect(listenerTransportCompatible("http", "tcp", "http://backup:8080")).toBe(false);
    expect(listenerTransportCompatible("http", "tcp", "tcp://backup:4444")).toBe(true);
  });
});

describe("canGeneratePayload", () => {
  it("requires a listener and a compatible transport", () => {
    expect(canGeneratePayload({ listenerId: "", listenerScheme: "http", beaconTransport: "http" })).toBe(false);
    expect(canGeneratePayload({ listenerId: "1", listenerScheme: "http", beaconTransport: "http" })).toBe(true);
    expect(canGeneratePayload({ listenerId: "1", listenerScheme: "http", beaconTransport: "tcp" })).toBe(false);
    expect(canGeneratePayload({
      listenerId: "1",
      listenerScheme: "http",
      beaconTransport: "tcp",
      failover: "tcp://10.0.0.1:4444",
    })).toBe(true);
  });
});
