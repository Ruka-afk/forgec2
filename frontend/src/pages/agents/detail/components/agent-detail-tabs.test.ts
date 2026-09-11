import { describe, expect, it } from "vitest";
import {
  AGENT_DETAIL_SECTION_TABS,
  AGENT_DETAIL_TABS,
  isAgentDetailTabId,
  sectionSupported,
  type AgentDetailSection,
} from "./agent-detail-tabs";

describe("agent-detail-tabs", () => {
  it("exposes six stable tabs", () => {
    expect(AGENT_DETAIL_TABS.map((t) => t.id)).toEqual([
      "overview",
      "tasks",
      "recon",
      "evasion",
      "collect",
      "timeline",
    ]);
  });

  it("validates tab ids from the URL", () => {
    expect(isAgentDetailTabId("recon")).toBe(true);
    expect(isAgentDetailTabId("nope")).toBe(false);
    expect(isAgentDetailTabId(null)).toBe(false);
    expect(isAgentDetailTabId(undefined)).toBe(false);
  });

  it("maps every section to a known tab", () => {
    const sections: AgentDetailSection[] = [
      "diagnose", "tasks", "hostinfo", "recon", "process", "evasion",
      "inject", "screenshots", "screentrigger", "registry", "browserhistory",
      "keylogger", "clipboard", "wallpaper", "webcammic", "timeline",
    ];
    for (const s of sections) {
      expect(isAgentDetailTabId(AGENT_DETAIL_SECTION_TABS[s])).toBe(true);
    }
  });

  it("supports everything on full implants", () => {
    const sections: AgentDetailSection[] = [
      "diagnose", "tasks", "hostinfo", "recon", "process", "evasion",
      "inject", "screenshots", "screentrigger", "registry", "browserhistory",
      "keylogger", "clipboard", "wallpaper", "webcammic", "timeline",
    ];
    for (const s of sections) {
      expect(sectionSupported(s, { isCImplant: false })).toBe(true);
    }
  });

  it("gates C implants to the core subset", () => {
    for (const s of ["diagnose", "tasks", "hostinfo", "process", "timeline"] as const) {
      expect(sectionSupported(s, { isCImplant: true })).toBe(true);
    }
    for (const s of [
      "recon", "evasion", "inject", "screenshots", "screentrigger",
      "registry", "browserhistory", "keylogger", "clipboard", "wallpaper", "webcammic",
    ] as const) {
      expect(sectionSupported(s, { isCImplant: true })).toBe(false);
    }
  });
});
