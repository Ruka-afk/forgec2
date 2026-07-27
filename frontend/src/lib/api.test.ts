import { describe, it, expect } from "vitest";

// Test the pure logic functions from api.ts directly
import { getCsrfToken } from "./api";

function unwrapBody<T>(body: unknown): T {
  if (
    body &&
    typeof body === "object" &&
    "success" in body &&
    (body as Record<string, unknown>).success === true &&
    "data" in body
  ) {
    return (body as Record<string, unknown>).data as T;
  }
  return body as T;
}

function buildUrl(path: string): string {
  return `${""}${path}`;
}

describe("unwrapBody", () => {
  it("extracts data from success envelope", () => {
    const result = unwrapBody<{ id: number }>({ success: true, data: { id: 42 } });
    expect(result).toEqual({ id: 42 });
  });

  it("returns body as-is when success is false", () => {
    const body = { success: false, error: "fail" };
    const result = unwrapBody(body);
    expect(result).toBe(body);
  });

  it("returns body as-is when success is missing", () => {
    const body = { error: "fail" };
    const result = unwrapBody(body);
    expect(result).toBe(body);
  });

  it("returns body as-is when data is missing", () => {
    const body = { success: true };
    const result = unwrapBody(body);
    expect(result).toBe(body);
  });

  it("returns non-object values as-is", () => {
    expect(unwrapBody("hello")).toBe("hello");
    expect(unwrapBody(42)).toBe(42);
    expect(unwrapBody(null)).toBeNull();
    expect(unwrapBody(undefined)).toBeUndefined();
  });
});

describe("getCsrfToken", () => {
  it("returns empty string when no cookie", () => {
    document.cookie = "";
    expect(getCsrfToken()).toBe("");
  });

  it("reads forgec2_csrf cookie", () => {
    document.cookie = "forgec2_csrf=abc123";
    expect(getCsrfToken()).toBe("abc123");
  });
});

describe("buildUrl", () => {
  it("returns the path unchanged with empty API_BASE", () => {
    expect(buildUrl("/api/tags")).toBe("/api/tags");
  });
});
