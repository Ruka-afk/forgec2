import { describe, it, expect } from "vitest";
import { emptyCredentialData, normalizeCredentialData } from "./types";

describe("normalizeCredentialData", () => {
  it("returns empty shape for null/undefined", () => {
    expect(normalizeCredentialData(null)).toEqual(emptyCredentialData());
    expect(normalizeCredentialData(undefined)).toEqual(emptyCredentialData());
  });

  it("prefers snake_case fields", () => {
    const r = normalizeCredentialData({
      vault_entries: [{ id: "1", username: "u" } as never],
      vault_count: 1,
      all_tags: ["a"],
    });
    expect(r.VaultCount).toBe(1);
    expect(r.VaultEntries).toHaveLength(1);
    expect(r.AllTags).toEqual(["a"]);
  });

  it("falls back to PascalCase", () => {
    const r = normalizeCredentialData({
      VaultEntries: [{ id: "2", username: "v" } as never],
      VaultCount: 3,
      AllTags: ["b", "c"],
    });
    expect(r.VaultCount).toBe(3);
    expect(r.VaultEntries[0].id).toBe("2");
    expect(r.AllTags).toEqual(["b", "c"]);
  });

  it("defaults missing arrays to empty", () => {
    const r = normalizeCredentialData({});
    expect(r.VaultEntries).toEqual([]);
    expect(r.VaultCount).toBe(0);
    expect(r.AllTags).toEqual([]);
  });
});
