import type { Listener, ProfilePreset } from "@/types/generate";

/** Normalize listeners list from /api/listeners envelopes. */
export function normalizeListeners(data: unknown): Listener[] {
  if (Array.isArray(data)) return data as Listener[];
  if (!data || typeof data !== "object") return [];
  const o = data as Record<string, unknown>;
  if (Array.isArray(o.listeners)) return o.listeners as Listener[];
  if (Array.isArray(o.data)) return o.data as Listener[];
  if (Array.isArray(o.Listeners)) return o.Listeners as Listener[];
  return [];
}

/** Extract profile presets from generate profiles API response. */
export function extractProfilePresets(
  profileData: unknown,
  fallback: ProfilePreset[],
): ProfilePreset[] {
  if (!profileData || typeof profileData !== "object") return fallback;
  const o = profileData as {
    success?: boolean;
    data?: { profiles?: ProfilePreset[] };
    profiles?: ProfilePreset[];
  };
  if (o.success && o.data?.profiles?.length) return o.data.profiles;
  if (Array.isArray(o.profiles) && o.profiles.length) return o.profiles;
  return fallback;
}

export function canStartGenerate(listenerId: string | undefined | null): boolean {
  return Boolean(listenerId && String(listenerId).trim());
}
