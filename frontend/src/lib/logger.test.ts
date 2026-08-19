import { describe, it, expect, vi, afterEach } from "vitest";
import { logger, redact } from "./logger";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("redact", () => {
  it("strips sensitive keys at any depth", () => {
    const value = {
      ok: "fine",
      token: "abc123",
      headers: { Authorization: "Bearer xyz", "X-CSRF-Token": "csrf" },
      nested: { password: "hunter2", apiKey: "k" },
    };
    const out = redact(value) as Record<string, unknown>;
    expect(out.ok).toBe("fine");
    expect(out.token).toBe("[redacted]");
    expect((out.headers as Record<string, unknown>).Authorization).toBe("[redacted]");
    expect((out.headers as Record<string, unknown>)["X-CSRF-Token"]).toBe("[redacted]");
    expect((out.nested as Record<string, unknown>).password).toBe("[redacted]");
    expect((out.nested as Record<string, unknown>).apiKey).toBe("[redacted]");
  });

  it("does not mutate the input", () => {
    const value = { token: "abc", name: "x" };
    redact(value);
    expect(value.token).toBe("abc");
  });

  it("passes through primitives and arrays", () => {
    expect(redact("hello")).toBe("hello");
    expect(redact([{ token: "t" }, 1])).toEqual([{ token: "[redacted]" }, 1]);
  });
});

describe("logger", () => {
  it("emits warn and error in every environment", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    logger.error("boom %s", "detail");
    expect(spy).toHaveBeenCalledWith("boom %s", "detail");
  });

  it("redacts sensitive fields before emitting", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    logger.error("request failed", { body: { token: "T", cmd: "ls" } });
    const arg = spy.mock.calls[0][1] as Record<string, unknown>;
    expect((arg.body as Record<string, unknown>).token).toBe("[redacted]");
  });

  it("scopes the prefix with withScope", () => {
    const spy = vi.spyOn(console, "warn").mockImplementation(() => {});
    logger.withScope("api").warn("retry", 2);
    expect(spy.mock.calls[0][0]).toBe("[api] retry");
    expect(spy.mock.calls[0][1]).toBe(2);
  });

  it("nested scopes stack", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    logger.withScope("api").withScope("poll").error("fail");
    expect(spy.mock.calls[0][0]).toBe("[api:poll] fail");
  });
});