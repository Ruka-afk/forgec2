import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

// Module-level AI status cache with 60s TTL so config changes propagate
// without a full page refresh. Lives in lib (not a page component) because
// several routes need it (agent detail card, tasks panels, macros dialog).
let aiStatusCache: { enabled: boolean; hasApiKey: boolean; ts: number } | null = null;
const AI_STATUS_TTL = 60_000;

export async function fetchAIStatus(): Promise<{ enabled: boolean; hasApiKey: boolean }> {
  if (aiStatusCache && Date.now() - aiStatusCache.ts < AI_STATUS_TTL) {
    return aiStatusCache;
  }
  try {
    const d = await api.get<{ enabled?: boolean; has_api_key?: boolean }>(paths.ai.status);
    aiStatusCache = { enabled: !!d.enabled && !!d.has_api_key, hasApiKey: !!d.has_api_key, ts: Date.now() };
  } catch {
    aiStatusCache = { enabled: false, hasApiKey: false, ts: Date.now() };
  }
  return aiStatusCache;
}
