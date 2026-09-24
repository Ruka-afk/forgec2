import { describe, expect, it } from "vitest";
import { bofImportFilename } from "./import-target";

describe("bofImportFilename", () => {
  it("derives the object name from the URL path", () => {
    expect(bofImportFilename("https://example.com/bofs/Timers.o")).toBe("Timers.o");
  });

  it("drops query strings and fragments", () => {
    expect(bofImportFilename("https://example.com/a/b/process.o?raw=1#L2")).toBe("process.o");
  });

  it("appends the .o extension when missing", () => {
    expect(bofImportFilename("https://example.com/bofs/Seatbelt")).toBe("Seatbelt.o");
  });

  it("falls back to a default name for extension-less URLs", () => {
    expect(bofImportFilename("https://example.com/")).toBe("download.o");
    expect(bofImportFilename("https://example.com")).toBe("download.o");
  });

  it("sanitizes unsafe characters and decodes escapes", () => {
    expect(bofImportFilename("https://example.com/a%20b.o")).toBe("a_b.o");
    expect(bofImportFilename("https://example.com/we;ird$.o")).toBe("we_ird_.o");
  });

  it("prefers an explicit name and keeps an existing .o suffix", () => {
    expect(bofImportFilename("https://example.com/x.o", "My BOF")).toBe("My_BOF.o");
    expect(bofImportFilename("https://example.com/x.o", "Custom.O")).toBe("Custom.O");
    expect(bofImportFilename("https://example.com/x.o", "   ")).toBe("x.o");
  });

  it("survives malformed URLs and malformed escapes", () => {
    expect(bofImportFilename("not a url")).toBe("not_a_url.o");
    expect(bofImportFilename("https://example.com/%zz.o")).toBe("_zz.o");
  });
});
