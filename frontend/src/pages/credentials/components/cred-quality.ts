export type CredActionQuality = "core" | "hardened" | "scripted" | "experimental";

interface CredDumpAction {
  action: string;
  quality: CredActionQuality;
  requiresMimikatzModule: boolean;
  /** Suffix under /agents/:id/ — empty means no dedicated dump route. */
  endpoint: string;
}

const CRED_DUMP_ACTIONS: CredDumpAction[] = [
  { action: "creds", quality: "scripted", requiresMimikatzModule: false, endpoint: "creds" },
  { action: "creds_dump", quality: "scripted", requiresMimikatzModule: false, endpoint: "creds" },
  { action: "hashdump", quality: "scripted", requiresMimikatzModule: false, endpoint: "creds" },
  { action: "mimikatz", quality: "scripted", requiresMimikatzModule: true, endpoint: "mimikatz" },
  { action: "dcsync", quality: "scripted", requiresMimikatzModule: true, endpoint: "mimikatz" },
  { action: "kerberoast", quality: "scripted", requiresMimikatzModule: false, endpoint: "kerberoast" },
  { action: "wifi_creds", quality: "hardened", requiresMimikatzModule: false, endpoint: "wifi_creds" },
  { action: "cookie_export", quality: "hardened", requiresMimikatzModule: false, endpoint: "cookie_export" },
  { action: "sccm_recon", quality: "scripted", requiresMimikatzModule: false, endpoint: "sccm_recon" },
  { action: "entra_prt", quality: "scripted", requiresMimikatzModule: false, endpoint: "entra_prt" },
];

export function moduleLooksLikeMimikatz(name: string): boolean {
  const n = name.trim().toLowerCase();
  return n === "invoke-mimikatz.ps1" || n === "mimikatz.ps1";
}

export function parseModuleNames(data: unknown): string[] {
  if (Array.isArray(data)) {
    return data.map((item) => (typeof item === "string" ? item : String((item as { name?: string }).name || "")));
  }
  if (!data || typeof data !== "object") return [];
  const rec = data as Record<string, unknown>;
  const list = rec.modules ?? rec.data;
  if (!Array.isArray(list)) return [];
  return list.map((item) => {
    if (typeof item === "string") return item;
    if (item && typeof item === "object") {
      const o = item as { name?: string; Name?: string };
      return String(o.name || o.Name || "");
    }
    return "";
  }).filter(Boolean);
}

export function hasMimikatzModule(modules: Array<{ name?: string } | string>): boolean {
  return modules.some((m) => moduleLooksLikeMimikatz(typeof m === "string" ? m : String(m.name || "")));
}

export function credActionDef(action: string): CredDumpAction | undefined {
  return CRED_DUMP_ACTIONS.find((a) => a.action === action);
}

export function credActionAllowed(action: string, hasModule: boolean): boolean {
  const def = credActionDef(action);
  if (!def) return true;
  if (!def.requiresMimikatzModule) return true;
  return hasModule;
}

export function credActionBlockReason(action: string, hasModule: boolean): "missing_module" | null {
  return credActionAllowed(action, hasModule) ? null : "missing_module";
}

export function credActionEndpoint(action: string): string {
  return credActionDef(action)?.endpoint || "";
}

/** Harvest buttons shown on the Credentials console (not a new page). */
export const CRED_HARVEST_ACTIONS: CredDumpAction[] = CRED_DUMP_ACTIONS.filter((a) =>
  a.action === "creds" || a.action === "mimikatz" || a.action === "kerberoast" || a.action === "wifi_creds"
  || a.action === "cookie_export" || a.action === "sccm_recon" || a.action === "entra_prt",
);
