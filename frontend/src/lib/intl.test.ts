import { describe, expect, it } from "vitest";
import { formatDateOnly, formatDateTime, formatNumber, formatTimeOnly, intlTag } from "./intl";

// Fixed local timestamp: the assertions compare against the same Intl call
// with an explicit tag, so they hold in any machine timezone.
const D = new Date(2026, 9, 17, 14, 5, 6);

describe("intl formatting", () => {
  it("maps UI locales to explicit BCP-47 tags", () => {
    expect(intlTag("en")).toBe("en-US");
    expect(intlTag("zh")).toBe("zh-CN");
  });

  it("formats dates in the requested locale, never the browser default", () => {
    expect(formatDateTime(D, "en")).toBe(D.toLocaleString("en-US"));
    expect(formatDateTime(D, "zh")).toBe(D.toLocaleString("zh-CN"));
    expect(formatDateOnly(D, "zh")).toBe(D.toLocaleDateString("zh-CN"));
  });

  it("formats time with explicit options", () => {
    expect(formatTimeOnly(D, { hour: "2-digit", minute: "2-digit" }, "zh")).toBe(
      D.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })
    );
  });

  it("formats numbers through the locale and keeps options", () => {
    expect(formatNumber(1234567.891, undefined, "en")).toBe((1234567.891).toLocaleString("en-US"));
    expect(formatNumber(1234.5678, { maximumFractionDigits: 2 }, "zh")).toBe(
      (1234.5678).toLocaleString("zh-CN", { maximumFractionDigits: 2 })
    );
  });

  it("degrades to a dash instead of Invalid Date", () => {
    expect(formatDateTime("definitely not a date")).toBe("-");
    expect(formatDateOnly(null)).toBe("-");
    expect(formatTimeOnly("")).toBe("-");
  });

  it("defaults to the active UI locale", () => {
    expect(formatDateTime(D)).toBe(formatDateTime(D, "en")); // provider default before hydration
  });
});
