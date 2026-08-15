export type DestQuality = "core" | "hardened" | "scripted" | "experimental";

export interface TokenActionDef {
  action: string;
  quality: DestQuality;
  /** Never true: steal/impersonate queue a Windows token task, they do not guarantee a CS-style store. */
  guaranteed: boolean;
  windowsPrimary: boolean;
}

export interface PersistenceMethodDef {
  key: string;
  quality: DestQuality;
  windowsPrimary: boolean;
}

export const TOKEN_ACTIONS: TokenActionDef[] = [
  { action: "steal", quality: "hardened", guaranteed: false, windowsPrimary: true },
  { action: "make", quality: "hardened", guaranteed: false, windowsPrimary: true },
  { action: "impersonate", quality: "hardened", guaranteed: false, windowsPrimary: true },
  { action: "revert", quality: "hardened", guaranteed: false, windowsPrimary: true },
  { action: "whoami", quality: "hardened", guaranteed: false, windowsPrimary: true },
];

export const PERSISTENCE_METHOD_QUALITY: PersistenceMethodDef[] = [
  { key: "registry", quality: "hardened", windowsPrimary: true },
  { key: "scheduled_task", quality: "hardened", windowsPrimary: true },
  { key: "startup_folder", quality: "hardened", windowsPrimary: true },
  { key: "service", quality: "hardened", windowsPrimary: true },
  { key: "wmi", quality: "scripted", windowsPrimary: true },
  { key: "image_file", quality: "scripted", windowsPrimary: true },
  { key: "com_hijack", quality: "experimental", windowsPrimary: true },
  { key: "dll_search_order", quality: "experimental", windowsPrimary: true },
];

export function tokenActionDef(action: string): TokenActionDef | undefined {
  return TOKEN_ACTIONS.find((a) => a.action === action);
}

export function tokenActionGuaranteed(action: string): boolean {
  return tokenActionDef(action)?.guaranteed === true;
}

export function tokenActionQuality(action: string): DestQuality | undefined {
  return tokenActionDef(action)?.quality;
}

export function persistenceMethodDef(key: string): PersistenceMethodDef | undefined {
  return PERSISTENCE_METHOD_QUALITY.find((m) => m.key === key);
}

export function persistenceMethodQuality(key: string): DestQuality | undefined {
  return persistenceMethodDef(key)?.quality;
}

export function persistenceLooksConfirmed(toastKey: "queued" | "installed"): boolean {
  return toastKey === "installed";
}
