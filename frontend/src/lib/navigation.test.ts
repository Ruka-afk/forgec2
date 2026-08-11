import { describe, expect, it } from "vitest";
import { NAV_ITEMS, NAV_SECTIONS, NAV_BY_HREF, NAV_SEGMENT_LABELS } from "@/lib/navigation";

describe("navigation single source", () => {
  it("nav items have unique hrefs and label keys", () => {
    const hrefs = NAV_ITEMS.map((i) => i.href);
    expect(new Set(hrefs).size).toBe(hrefs.length);
    const labels = NAV_ITEMS.map((i) => i.labelKey);
    expect(new Set(labels).size).toBe(labels.length);
    expect(NAV_ITEMS.length).toBeGreaterThan(30);
  });

  it("every item label key uses the nav.* namespace", () => {
    for (const item of NAV_ITEMS) {
      expect(item.labelKey).toMatch(/^nav\./);
    }
  });

  it("NAV_BY_HREF maps every top-level page", () => {
    for (const item of NAV_ITEMS) {
      expect(NAV_BY_HREF[item.href]).toBe(item.labelKey);
    }
  });

  it("NAV_SEGMENT_LABELS covers every href normalized for breadcrumbs", () => {
    for (const item of NAV_ITEMS) {
      const seg = item.href.replace(/^\//, "").replace(/-/g, "_");
      expect(NAV_SEGMENT_LABELS[seg]).toBe(item.labelKey);
    }
  });

  it("sections are non-empty and preserve sidebar order groups", () => {
    expect(NAV_SECTIONS.map((s) => s.titleKey)).toEqual([
      "operations",
      "build-deploy",
      "post-exploitation",
      "intel-analysis",
      "lab",
      "system",
    ]);
    for (const section of NAV_SECTIONS) {
      expect(section.items.length).toBeGreaterThan(0);
    }
  });
});
