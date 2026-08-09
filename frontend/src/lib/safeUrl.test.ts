import { describe, expect, it } from "vitest";
import { isSafeUrl, safeHref, isSafeImageSrc, safeImageSrc } from "./safeUrl";

describe("isSafeUrl", () => {
  it("accepts relative paths", () => {
    expect(isSafeUrl("/agents/abc")).toBe(true);
    expect(isSafeUrl("/screenshots/agent1/shot.png")).toBe(true);
  });

  it("rejects protocol-relative and backslash tricks", () => {
    expect(isSafeUrl("//evil.example")).toBe(false);
    expect(isSafeUrl("//evil.example/steal")).toBe(false);
    expect(isSafeUrl("/\\evil.example")).toBe(false);
    expect(isSafeUrl("\\evil.example")).toBe(false);
  });

  it("accepts explicit http(s) URLs", () => {
    expect(isSafeUrl("https://github.com/forgec2/forgec2/releases")).toBe(true);
    expect(isSafeUrl("http://example.com/dl")).toBe(true);
  });

  it("rejects dangerous schemes and junk", () => {
    expect(isSafeUrl("javascript:alert(1)")).toBe(false);
    expect(isSafeUrl("JAVASCRIPT:alert(1)")).toBe(false);
    expect(isSafeUrl("data:text/html,<script>1</script>")).toBe(false);
    expect(isSafeUrl("vbscript:msgbox(1)")).toBe(false);
    expect(isSafeUrl("file:///etc/passwd")).toBe(false);
    expect(isSafeUrl("")).toBe(false);
    expect(isSafeUrl(123)).toBe(false);
    expect(isSafeUrl(null)).toBe(false);
    expect(isSafeUrl(undefined)).toBe(false);
  });

  it("rejects oversized values", () => {
    expect(isSafeUrl("a".repeat(5000))).toBe(false);
  });
});

describe("safeHref", () => {
  it("returns the value when safe, undefined otherwise", () => {
    expect(safeHref("/agents/x")).toBe("/agents/x");
    expect(safeHref("javascript:alert(1)")).toBeUndefined();
  });
});

describe("isSafeImageSrc / safeImageSrc", () => {
  it("allows data:image/* plus safe urls", () => {
    expect(isSafeImageSrc("data:image/png;base64,AAAA")).toBe(true);
    expect(isSafeImageSrc("data:image/jpeg;base64,BBBB")).toBe(true);
    expect(safeImageSrc("/screenshots/a.png")).toBe("/screenshots/a.png");
  });

  it("rejects html/svg data urls and scripts", () => {
    expect(isSafeImageSrc("data:text/html,<svg onload=alert(1)>")).toBe(false);
    expect(isSafeImageSrc("data:image/svg+xml,<svg onload=alert(1)>")).toBe(false);
    expect(isSafeImageSrc("javascript:alert(1)")).toBe(false);
    expect(safeImageSrc("javascript:alert(1)")).toBeUndefined();
  });

  it("rejects oversized images", () => {
    expect(isSafeImageSrc("a".repeat(20 * 1024 * 1024))).toBe(false);
  });
});