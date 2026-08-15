import type { Screenshot } from "@/types/screenshot";
import type { KeylogTask, DownloadTask, LootData } from "@/types/loot";

export type LootTab = "screenshots" | "keylogs" | "downloads";

export function emptyLootData(): LootData {
  return { screenshots: [], keylog_tasks: [], download_tasks: [] };
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

function asList(v: unknown): unknown[] {
  return Array.isArray(v) ? v : [];
}

export function normalizeScreenshot(raw: unknown): Screenshot | null {
  const o = asRecord(raw);
  if (!o) return null;
  const agentId = firstStr(o, ["agent_id", "AgentID"]);
  const filename = firstStr(o, ["filename", "Filename"]);
  const path = firstStr(o, ["path", "Path"], agentId && filename ? `${agentId}/${filename}` : "");
  if (!agentId && !filename && !path) return null;
  const id = firstStr(o, ["id", "ID"], agentId && filename ? `screenshot:${agentId}:${filename}` : path);
  return {
    id,
    agent_id: agentId,
    filename,
    path,
    created_at: firstStr(o, ["created_at", "CreatedAt"]),
  };
}

function normalizeKeylog(raw: unknown): KeylogTask | null {
  const o = asRecord(raw);
  if (!o) return null;
  const id = firstStr(o, ["id", "ID"]);
  if (!id) return null;
  const agent = asRecord(o.agent ?? o.Agent);
  return {
    id,
    agent_id: firstStr(o, ["agent_id", "AgentID"]),
    hostname: firstStr(o, ["hostname", "Hostname"]),
    agent: agent ? { hostname: firstStr(agent, ["hostname", "Hostname"]) } : undefined,
    result: firstStr(o, ["result", "Result"]),
    error: firstStr(o, ["error", "Error"]),
    status: firstStr(o, ["status", "Status"]),
    created_at: firstStr(o, ["created_at", "CreatedAt"]),
  };
}

function normalizeDownload(raw: unknown): DownloadTask | null {
  const o = asRecord(raw);
  if (!o) return null;
  const id = firstStr(o, ["id", "ID"]);
  if (!id) return null;
  const agent = asRecord(o.agent ?? o.Agent);
  return {
    id,
    agent_id: firstStr(o, ["agent_id", "AgentID"]),
    hostname: firstStr(o, ["hostname", "Hostname"]),
    agent: agent ? { hostname: firstStr(agent, ["hostname", "Hostname"]) } : undefined,
    type: firstStr(o, ["type", "Type"]),
    command: firstStr(o, ["command", "Command"]),
    result: firstStr(o, ["result", "Result"]),
    status: firstStr(o, ["status", "Status"]),
    created_at: firstStr(o, ["created_at", "CreatedAt"]),
  };
}

export function lootDeleteId(
  kind: "screenshot" | "keylog" | "download",
  item: { id: string; agent_id?: string; filename?: string },
): string {
  const id = String(item.id || "");
  if (id.startsWith(`${kind}:`)) return id;
  if (kind === "screenshot") return `screenshot:${item.agent_id || ""}:${item.filename || ""}`;
  return `${kind}:${id}`;
}

export function normalizeLootData(result: Record<string, unknown> | null | undefined): LootData {
  if (!result) return emptyLootData();
  return {
    screenshots: asList(result.screenshots ?? result.Screenshots).map(normalizeScreenshot).filter((s): s is Screenshot => s != null),
    keylog_tasks: asList(result.keylog_tasks ?? result.keylogs ?? result.KeylogTasks).map(normalizeKeylog).filter((k): k is KeylogTask => k != null),
    download_tasks: asList(result.download_tasks ?? result.downloads ?? result.DownloadTasks).map(normalizeDownload).filter((d): d is DownloadTask => d != null),
  };
}
