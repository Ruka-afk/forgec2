export type SessionActionQuality = "core" | "hardened" | "scripted" | "experimental";

export interface SessionActionDef {
  action: string;
  quality: SessionActionQuality;
  /** True only for an actual elevate/bypass attempt, never for a recon check. */
  isElevate: boolean;
}

export const SESSION_POSTEX_ACTIONS: SessionActionDef[] = [
  { action: "shell", quality: "core", isElevate: false },
  { action: "ps", quality: "core", isElevate: false },
  { action: "screenshot", quality: "hardened", isElevate: false },
  { action: "screen", quality: "hardened", isElevate: false },
  { action: "clipboard_get", quality: "hardened", isElevate: false },
  { action: "privesc_check", quality: "hardened", isElevate: false },
  { action: "keylogger_start", quality: "hardened", isElevate: false },
  { action: "keylogger_stop", quality: "hardened", isElevate: false },
  { action: "keylogger_dump", quality: "hardened", isElevate: false },
  { action: "hashdump", quality: "scripted", isElevate: false },
  { action: "creds_dump", quality: "scripted", isElevate: false },
  { action: "mimikatz", quality: "scripted", isElevate: false },
  { action: "remote-desktop", quality: "experimental", isElevate: false },
  { action: "elevate", quality: "hardened", isElevate: true },
  { action: "uac_bypass", quality: "hardened", isElevate: true },
];

export function sessionActionDef(action: string): SessionActionDef | undefined {
  return SESSION_POSTEX_ACTIONS.find((a) => a.action === action);
}

export function sessionActionQuality(action: string): SessionActionQuality | undefined {
  return sessionActionDef(action)?.quality;
}

/** Recon checks must never be treated as a guaranteed elevation. */
export function isGuaranteedElevate(action: string): boolean {
  return sessionActionDef(action)?.isElevate === true;
}

export function isExperimentalDesktop(action: string): boolean {
  return sessionActionQuality(action) === "experimental";
}
