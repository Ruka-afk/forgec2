import { describe, expect, it } from "vitest";
import { formatFromOs, parseGenerateQuery, rebuildPayloadHref } from "./generate-query";

describe("formatFromOs", () => {
  it("maps os names to generate tabs", () => {
    expect(formatFromOs("windows")).toBe("exe");
    expect(formatFromOs("Linux")).toBe("linux");
    expect(formatFromOs("darwin")).toBe("macos");
    expect(formatFromOs("unknown")).toBeUndefined();
  });
});

describe("parseGenerateQuery", () => {
  it("reads listener_id, format, os, and arch", () => {
    const q = parseGenerateQuery("?listener_id=12&format=linux&arch=arm64");
    expect(q.listenerId).toBe("12");
    expect(q.format).toBe("linux");
    expect(q.arch).toBe("arm64");
  });
  it("derives format from os when format is omitted", () => {
    expect(parseGenerateQuery("os=windows&arch=amd64").format).toBe("exe");
  });
  it("ignores a missing listener", () => {
    expect(parseGenerateQuery("").listenerId).toBeUndefined();
  });
});

describe("rebuildPayloadHref", () => {
  it("builds a generate deep link from a beacon", () => {
    expect(rebuildPayloadHref({ listener_id: 3, os: "windows", arch: "amd64" }))
      .toBe("/generate?listener_id=3&os=windows&arch=amd64&format=exe");
  });
  it("omits a missing listener", () => {
    expect(rebuildPayloadHref({ os: "linux" })).toBe("/generate?os=linux&format=linux");
  });
  it("returns /generate when nothing is known", () => {
    expect(rebuildPayloadHref({})).toBe("/generate");
  });
});
