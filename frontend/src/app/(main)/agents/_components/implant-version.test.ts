import { describe, expect, it } from "vitest";
import { destNeedsKnownVersion, hostImplantVersions, implantBlocksDest, knownImplantVersion } from "./implant-version";

describe("knownImplantVersion", () => {
  it("treats blank values as unknown", () => {
    expect(knownImplantVersion("2.4.1")).toBe("2.4.1");
    expect(knownImplantVersion("  ")).toBe("");
    expect(knownImplantVersion(undefined)).toBe("");
  });
});

describe("hostImplantVersions", () => {
  it("dedupes reported versions and drops blanks", () => {
    expect(hostImplantVersions([{ version: "2.4.1" }, { version: "2.4.1" }, { version: "" }])).toBe("2.4.1");
    expect(hostImplantVersions([{ version: "1.0" }, { version: "2.0" }])).toBe("1.0, 2.0");
    expect(hostImplantVersions([{ version: "" }, {}])).toBe("");
  });
});

describe("implantBlocksDest", () => {
  it("blocks scripted and experimental dests when the implant never reported a version", () => {
    expect(destNeedsKnownVersion("scripted")).toBe(true);
    expect(destNeedsKnownVersion("experimental")).toBe(true);
    expect(destNeedsKnownVersion("hardened")).toBe(false);
    expect(destNeedsKnownVersion("core")).toBe(false);
    expect(implantBlocksDest("", "scripted")).toBe(true);
    expect(implantBlocksDest("  ", "experimental")).toBe(true);
    expect(implantBlocksDest("2.4.1", "scripted")).toBe(false);
    expect(implantBlocksDest("", "hardened")).toBe(false);
    expect(implantBlocksDest("", "core")).toBe(false);
  });
});
