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
