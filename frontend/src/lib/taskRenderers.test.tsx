import { describe, it, expect, beforeEach } from "vitest";
import { renderToString } from "react-dom/server";
import {
  registerTaskRenderer,
  getTaskRenderer,
  registeredRendererTypes,
  HostInfoResultView,
} from "./taskRenderers";

describe("taskRenderers registry", () => {
  beforeEach(() => {
    // Registry is module-global; clear probes between tests via overwrite.
  });

  it("registers and resolves renderers by task type", () => {
    const Probe = () => null;
    registerTaskRenderer("zz-probe", Probe);
    expect(getTaskRenderer("zz-probe")).toBe(Probe);
    expect(registeredRendererTypes()).toContain("zz-probe");
  });

  it("returns undefined for unregistered types (text fallback contract)", () => {
    expect(getTaskRenderer("no-such-type")).toBeUndefined();
  });

  it("later registrations override earlier ones", () => {
    const A = () => null;
    const B = () => null;
    registerTaskRenderer("zz-override", A);
    registerTaskRenderer("zz-override", B);
    expect(getTaskRenderer("zz-override")).toBe(B);
    registerTaskRenderer("zz-override", A); // restore for other assertions
  });
});

describe("HostInfoResultView", () => {
  const good = JSON.stringify({
    platform: "windows",
    collected_at: "2026-01-01T00:00:00Z",
    sections: {
      security: {
        av_products: [{ name: "Defender", protection: "enabled", signatures: "up_to_date", state_hex: "0x061100" }],
        edr_processes: [],
      },
      system: { username: "alice", integrity: "High" },
    },
  });

  it("renders structured sections with AV state badges", () => {
    const html = renderToString(<HostInfoResultView result={good} taskType="hostinfo" />);
    expect(html).toContain("hostinfo-renderer");
    expect(html).toContain("Defender");
    expect(html).toContain("enabled");
    expect(html).toContain("security");
  });

  it("falls back to raw text when the payload is not hostinfo JSON", () => {
    const html = renderToString(
      <HostInfoResultView result="plain shell output" taskType="hostinfo" />,
    );
    expect(html).toContain("<pre");
    expect(html).toContain("plain shell output");
  });
});
