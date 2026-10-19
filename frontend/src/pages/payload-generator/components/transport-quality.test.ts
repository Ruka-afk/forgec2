import { describe, expect, it } from "vitest";
import {
  BEACON_TRANSPORTS,
  isExperimentalTransport,
  qualityLabelKey,
  transportQuality,
  visibleBeaconTransports,
} from "./transport-quality";

describe("transportQuality", () => {
  it("does not treat experimental transports as core", () => {
    expect(transportQuality("http")).toBe("core");
    expect(transportQuality("tcp")).toBe("core");
    expect(transportQuality("dns")).toBe("hardened");
    expect(transportQuality("wss")).toBe("hardened");
    expect(transportQuality("grpc")).toBe("experimental");
    expect(transportQuality("ssh")).toBe("experimental");
    expect(transportQuality("icmp")).toBe("experimental");
    expect(transportQuality("mtls")).toBe("experimental");
    expect(transportQuality("h2c")).toBe("experimental");
    expect(isExperimentalTransport("http")).toBe(false);
    expect(isExperimentalTransport("grpc")).toBe(true);
  });

  it("hides experimental options until expanded", () => {
    const coreish = visibleBeaconTransports(false);
    expect(coreish.every((t) => t.quality !== "experimental")).toBe(true);
    expect(coreish.map((t) => t.value)).toEqual(["http", "tcp", "dns", "wss"]);
    const all = visibleBeaconTransports(true);
    expect(all).toHaveLength(BEACON_TRANSPORTS.length);
    expect(all.some((t) => t.value === "icmp")).toBe(true);
    expect(qualityLabelKey("experimental")).toBe("generate.quality_experimental");
  });
});
