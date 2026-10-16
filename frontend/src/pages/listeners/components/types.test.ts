import { describe, expect, it } from "vitest";
import { emptyCreateForm, emptyEditForm, isTLSScheme } from "./types";

describe("isTLSScheme", () => {
  it("accepts the two schemes that terminate TLS, case-insensitively", () => {
    expect(isTLSScheme("https")).toBe(true);
    expect(isTLSScheme("TLS")).toBe(true);
    expect(isTLSScheme(" https ")).toBe(true);
  });

  it("rejects plain and non-terminating schemes, including empty values", () => {
    for (const scheme of ["http", "tcp", "dns", "icmp", "h2c", "ws", "", undefined, null]) {
      expect(isTLSScheme(scheme)).toBe(false);
    }
  });

  it("keeps the TLS posture out of a fresh form, so a create cannot inherit it", () => {
    const form = emptyCreateForm();
    expect(form.client_ca_file).toBe("");
    expect(form.require_client_cert).toBe(false);
    expect(form.min_tls_version).toBe("");
    expect(emptyEditForm().require_client_cert).toBe(false);
  });
});
