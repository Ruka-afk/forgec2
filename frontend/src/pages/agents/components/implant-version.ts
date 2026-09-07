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
