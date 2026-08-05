export interface VaultEntry {
  id: string;
  domain: string;
  username: string;
  password: string;
  hash: string;
  type: string;
  source: string;
  tags: string;
  expires_at: string;
  confirmed: boolean;
  agent_id: string;
  notes: string;
}

export interface CredentialData {
  VaultEntries: VaultEntry[];
  VaultCount: number;
  AllTags: string[];
}

export const CRED_TYPES = ["all", "password", "hash", "token", "key", "ntlm", "kerberos", "cleartext"];

export const TYPE_BADGE_VARIANT: Record<string, "success" | "warning" | "outline"> = {
  cleartext: "success",
  password: "success",
  ntlm: "warning",
  hash: "warning",
  token: "outline",
  key: "outline",
  kerberos: "outline",
  sha1: "warning",
};

export const emptyCredentialData = (): CredentialData => ({
  VaultEntries: [],
  VaultCount: 0,
  AllTags: [],
});

/** Normalize credentials API envelope (snake_case / PascalCase). */
export function normalizeCredentialData(result: {
  VaultEntries?: VaultEntry[];
  vault_entries?: VaultEntry[];
  VaultCount?: number;
  vault_count?: number;
  AllTags?: string[];
  all_tags?: string[];
} | null | undefined): CredentialData {
  if (!result) return emptyCredentialData();
  return {
    VaultEntries: result.vault_entries || result.VaultEntries || [],
    VaultCount: result.vault_count ?? result.VaultCount ?? 0,
    AllTags: result.all_tags || result.AllTags || [],
  };
}
