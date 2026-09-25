import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

// Module-level AI status cache with 60s TTL so config changes propagate
// without a full page refresh. Lives in lib (not a page component) because
// several routes need it (agent detail card, tasks panels, macros dialog).
//
// Only SUCCESSFUL reads are cached. A failed query used to be cached as
// {enabled:false}, which for the next 60s made four AI features disappear as
// though they had been deliberately switched off.
type CachedStatus = { enabled: boolean; hasApiKey: boolean; ts: number };
let aiStatusCache: CachedStatus | null = null;
const AI_STATUS_TTL = 60_000;

export type AIStatus = {
  /** AI is configured AND has a key — safe to offer the feature. */
  enabled: boolean;
  hasApiKey: boolean;
  /** False when the status query failed: `enabled` is then not a real answer. */
  known: boolean;
  error?: string;
};

export async function fetchAIStatus(): Promise<AIStatus> {
  if (aiStatusCache && Date.now() - aiStatusCache.ts < AI_STATUS_TTL) {
    return { ...aiStatusCache, known: true };
  }
  try {
    const d = await api.get<{ enabled?: boolean; has_api_key?: boolean }>(paths.ai.status);
    aiStatusCache = { enabled: !!d.enabled && !!d.has_api_key, hasApiKey: !!d.has_api_key, ts: Date.now() };
    return { ...aiStatusCache, known: true };
  } catch (e) {
    // Unknown, not disabled. Do not poison the cache: the next caller retries.
    return {
      enabled: false,
      hasApiKey: false,
      known: false,
      error: e instanceof Error ? e.message : undefined,
    };
  }
}

/** Drop the cache so the next fetchAIStatus() re-reads immediately. */
export function invalidateAIStatus(): void {
  aiStatusCache = null;
}
