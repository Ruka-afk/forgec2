/** Empty or whitespace version means the implant never reported one. */
export function knownImplantVersion(version?: string | null): string {
  return (version || "").trim();
}

/** Scripted / experimental dests are newer than a blank version can promise. */
export function destNeedsKnownVersion(quality?: string | null): boolean {
  return quality === "scripted" || quality === "experimental";
}

export function implantBlocksDest(version?: string | null, quality?: string | null): boolean {
  return destNeedsKnownVersion(quality) && !knownImplantVersion(version);
}

/** C prototype implant (version c-*) supports only shell/ps/ls/read/hostinfo/
 * set_sleep/beacon_now/kill/download/upload/download_url. Everything else
 * (screenshots, keylogger, registry, BOF, tokens, …) needs the Go implant. */
export function isCImplant(version?: string | null): boolean {
  return knownImplantVersion(version).toLowerCase().startsWith("c-");
}


