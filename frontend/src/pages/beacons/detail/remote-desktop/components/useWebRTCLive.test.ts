import { describe, expect, it } from "vitest";
import { LIVE_PRESETS, buildOperatorWsUrl, getLivePreset } from "./useWebRTCLive";

describe("WebRTC live helpers", () => {
  it("builds the authenticated operator signaling URL for both schemes", () => {
    expect(
      buildOperatorWsUrl("session abc", { protocol: "https:", host: "c2.example:8443" }),
    ).toBe("wss://c2.example:8443/ws/rd/operator?session=session%20abc");
    expect(
      buildOperatorWsUrl("s1", { protocol: "http:", host: "127.0.0.1:8000" }),
    ).toBe("ws://127.0.0.1:8000/ws/rd/operator?session=s1");
  });

  it("offers 720p30 as the baseline and 1080p30 as the ceiling", () => {
    expect(LIVE_PRESETS.map((preset) => preset.value)).toEqual(["720p", "1080p"]);
    expect(LIVE_PRESETS[0]).toMatchObject({ width: 1280, height: 720, fps: 30 });
    expect(LIVE_PRESETS[1]).toMatchObject({ width: 1920, height: 1080, fps: 30 });
    for (const preset of LIVE_PRESETS) {
      expect(preset.width * preset.height).toBeLessThanOrEqual(1920 * 1080);
      expect(preset.bitrateKbps).toBeGreaterThan(0);
    }
  });

  it("falls back to 720p for an unknown preset", () => {
    expect(getLivePreset("4k").value).toBe("720p");
    expect(getLivePreset("1080p").value).toBe("1080p");
  });
});
