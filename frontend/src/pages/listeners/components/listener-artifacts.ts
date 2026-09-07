interface ListenerArtifact {
  id: string;
  filename: string;
  format: string;
  platform: string;
  status: string;
  created_at: string;
  listener_id: string;
}

function asRecord(v: unknown): Record<string, unknown> | null {
  if (!v || typeof v !== "object" || Array.isArray(v)) return null;
  return v as Record<string, unknown>;
}

function firstStr(obj: Record<string, unknown>, keys: string[], fallback = ""): string {
  for (const k of keys) {
    if (obj[k] != null && obj[k] !== "") return String(obj[k]);
  }
  return fallback;
}

export function normalizeBuildLog(raw: unknown): ListenerArtifact | null {
  const o = asRecord(raw);
  if (!o) return null;
  const id = firstStr(o, ["id", "ID"]);
  if (!id) return null;
  return {
    id,
    filename: firstStr(o, ["filename", "Filename"]),
    format: firstStr(o, ["format", "Format"]),
    platform: firstStr(o, ["platform", "Platform"]),
    status: firstStr(o, ["status", "Status"]),
    created_at: firstStr(o, ["created_at", "CreatedAt"]),
    listener_id: firstStr(o, ["listener_id", "ListenerID"]),
  };
}

export function artifactsForListener(raw: unknown, listenerId: string): ListenerArtifact[] {
  const list = Array.isArray(raw) ? raw : [];
  const want = String(listenerId || "");
  return list
    .map(normalizeBuildLog)
    .filter((row): row is ListenerArtifact => row != null)
    .filter((row) => want !== "" && row.listener_id === want);
}

export function artifactDownloadable(status: string): boolean {
  const s = status.toLowerCase();
  return s === "success" || s === "completed";
}
