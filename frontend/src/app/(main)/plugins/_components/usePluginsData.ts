"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { normalizePluginList, type Plugin } from "./types";

export function usePluginsData() {
  const { t } = useI18n();
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadPlugins = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      const apiData = await api.get(paths.plugins.list, { signal });
      if (signal?.aborted) return;
      setPlugins(normalizePluginList(apiData));
    } catch (e) {
      if (signal?.aborted) return;
      setPlugins([]);
      const msg = e instanceof Error ? e.message : t("plugins.toast.load_failed");
      setError(msg);
      toast.error(msg);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    const ac = new AbortController();
    void loadPlugins(ac.signal);
    return () => ac.abort();
  }, [loadPlugins]);

  return {
    plugins,
    setPlugins,
    loading,
    error,
    loadPlugins: () => loadPlugins(),
    reload: () => loadPlugins(),
  };
}
