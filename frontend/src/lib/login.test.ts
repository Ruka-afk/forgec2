import { describe, it, expect } from "vitest";
import { isLoginSuccessResponse, parseLoginErrorBody, safeNextPath } from "./login";

describe("isLoginSuccessResponse", () => {
  it("accepts 302/303/307", () => {
    expect(isLoginSuccessResponse({ status: 302, type: "basic", ok: false })).toBe(true);
    expect(isLoginSuccessResponse({ status: 303, type: "basic", ok: false })).toBe(true);
    expect(isLoginSuccessResponse({ status: 307, type: "basic", ok: false })).toBe(true);
  });

  it("accepts opaque redirect (status 0)", () => {
    expect(isLoginSuccessResponse({ status: 0, type: "opaqueredirect", ok: false })).toBe(true);
    expect(isLoginSuccessResponse({ status: 0, type: "opaque", ok: false })).toBe(true);
  });

  it("rejects 401 and generic 200", () => {
    expect(isLoginSuccessResponse({ status: 401, type: "basic", ok: false })).toBe(false);
    expect(isLoginSuccessResponse({ status: 200, type: "basic", ok: true })).toBe(false);
    expect(isLoginSuccessResponse({ status: 204, type: "basic", ok: true })).toBe(false);
  });

  it("rejects bare status 0 without opaque type", () => {
    expect(isLoginSuccessResponse({ status: 0, type: "error", ok: false })).toBe(false);
  });
});

describe("parseLoginErrorBody", () => {
  it("reads error field", () => {
    expect(parseLoginErrorBody({ error: "Invalid username or password" })).toBe(
      "Invalid username or password",
    );
  });

  it("surfaces 2FA required", () => {
    expect(parseLoginErrorBody({ require_totp: true })).toBe("Two-factor authentication required");
    expect(parseLoginErrorBody({ require_2fa: true, error: "2FA needed" })).toBe("2FA needed");
  });

  it("returns null for empty body", () => {
    expect(parseLoginErrorBody(null)).toBeNull();
    expect(parseLoginErrorBody({})).toBeNull();
  });
});

describe("safeNextPath", () => {
  it("allows relative app paths", () => {
    expect(safeNextPath("/agents")).toBe("/agents");
    expect(safeNextPath("/agents/abc/shell")).toBe("/agents/abc/shell");
  });

  it("rejects open redirects", () => {
    expect(safeNextPath("//evil.com")).toBe("/dashboard");
    expect(safeNextPath("/\\evil.com")).toBe("/dashboard");
    expect(safeNextPath("https://evil.com")).toBe("/dashboard");
    expect(safeNextPath("http://evil.com")).toBe("/dashboard");
  });

  it("rejects login loop and empty", () => {
    expect(safeNextPath("/login")).toBe("/dashboard");
    expect(safeNextPath(null)).toBe("/dashboard");
    expect(safeNextPath("")).toBe("/dashboard");
  });

  it("rejects encoded protocol tricks", () => {
    expect(safeNextPath("/%2f%2fevil.com")).toBe("/dashboard");
  });
});
