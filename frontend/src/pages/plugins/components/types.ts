export interface Plugin {
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  Version?: string;
  version?: string;
  Description?: string;
  description?: string;
  Author?: string;
  author?: string;
  Category?: string;
  category?: string;
  PluginType?: string;
  plugin_type?: string;
  Enabled?: boolean;
  enabled?: boolean;
  Rating?: number;
  rating?: number;
  RatingOverall?: number;
  rating_overall?: number;
  Dependencies?: string[];
  dependencies?: string[];
  Installed?: boolean;
  installed?: boolean;
  UpdateAvailable?: boolean;
  update_available?: boolean;
  LastUpdated?: string;
  last_updated?: string;
  Icon?: string;
  icon?: string;
  Readme?: string;
  readme?: string;
  Size?: number;
  size?: number;
  Downloads?: number;
  downloads?: number;
}

export interface Review {
  id?: string;
  user?: string;
  username?: string;
  rating?: number;
  content?: string;
  comment?: string;
  created_at?: string;
}

export function pluginId(p: Plugin): string {
  return String(p.ID || p.id || "");
}

/**
 * The backend stores Dependencies as a JSON-encoded TEXT column, so the API
 * may return either a real array or the raw string '["a","b"]'. Normalize
 * both (plus comma-separated fallbacks) into a string array.
 */
export function normalizePluginDeps(raw: unknown): string[] {
  if (Array.isArray(raw)) return raw.map(String).filter(Boolean);
  if (typeof raw === "string" && raw.trim() !== "") {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) return parsed.map(String).filter(Boolean);
    } catch { /* fall through to plain-text handling */ }
    return raw.split(",").map((d) => d.trim()).filter(Boolean);
  }
  return [];
}

/** Overall rating across the snake_case/camelCase variants the API emits. */
export function pluginRating(p: Plugin): number {
  return Number(p.rating_overall ?? p.RatingOverall ?? p.rating ?? p.Rating ?? 0) || 0;
}

export function normalizePluginList(apiData: unknown): Plugin[] {
  if (Array.isArray(apiData)) return apiData as Plugin[];
  if (!apiData || typeof apiData !== "object") return [];
  const o = apiData as { plugins?: Plugin[]; Plugins?: Plugin[] };
  return o.plugins || o.Plugins || [];
}
