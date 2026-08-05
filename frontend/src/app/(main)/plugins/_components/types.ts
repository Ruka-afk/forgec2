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
  Dependencies?: string[];
  dependencies?: string[];
  Installed?: boolean;
  installed?: string;
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
  rating?: number;
  content?: string;
  created_at?: string;
}

export function pluginId(p: Plugin): string {
  return String(p.ID || p.id || "");
}

export function normalizePluginList(apiData: unknown): Plugin[] {
  if (Array.isArray(apiData)) return apiData as Plugin[];
  if (!apiData || typeof apiData !== "object") return [];
  const o = apiData as { plugins?: Plugin[]; Plugins?: Plugin[] };
  return o.plugins || o.Plugins || [];
}
