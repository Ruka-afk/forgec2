import { describe, expect, it } from "vitest";
import { auditSessionId, looksLikeAgentId, normalizeAuditLog, normalizeAuditLogs } from "./audit-log";

describe("normalizeAuditLog", () => {
  it("maps teamserver fields onto the operator row", () => {
    const row = normalizeAuditLog({
      id: 9,
      user: "alice",
      action: "file_upload_push",
      resource: "agent",
      agent_id: "11111111-2222-4333-8444-555555555555",
      ip: "10.0.0.8",
      success: true,
      details: "C:\\secret.txt",
      created_at: "2026-08-14T12:00:00Z",
    });
    expect(row).toMatchObject({
      id: "9",
      username: "alice",
      timestamp: "2026-08-14T12:00:00Z",
      status: "success",
      target: "11111111-2222-4333-8444-555555555555",
      agent_id: "11111111-2222-4333-8444-555555555555",
      severity: "info",
    });
  });

  it("marks failed actions", () => {
    expect(normalizeAuditLog({ success: false, action: "login" })?.status).toBe("failed");
    expect(normalizeAuditLog({ success: false })?.severity).toBe("error");
  });
});

describe("auditSessionId", () => {
  it("prefers agent_id and only guesses UUID targets", () => {
    expect(auditSessionId({ agent_id: "abc" })).toBe("abc");
    expect(auditSessionId({ target: "11111111-2222-4333-8444-555555555555" })).toBe("11111111-2222-4333-8444-555555555555");
    expect(auditSessionId({ target: "login" })).toBe("");
    expect(looksLikeAgentId("not-a-uuid")).toBe(false);
  });
});

describe("normalizeAuditLogs", () => {
  it("drops junk rows", () => {
    expect(normalizeAuditLogs([null, { id: 1, user: "a" }])).toHaveLength(1);
  });
});
