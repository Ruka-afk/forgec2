"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import type { LootData } from "@/types/loot";
import { emptyLootData, normalizeLootData } from "./types";

export function useLootData() {
  const { t } = useI18n();
  const [data, setData] = useState<LootData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadLoot = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      // Dual-use /loot (HTML page vs JSON Accept) — not under /api prefix
      const result = await api.get(paths.loot.page, { signal });
      setData(normalizeLootData(result as Record<string, unknown>));
    } catch (e) {
      if (signal?.aborted) return;
      setData(emptyLootData());
      const msg = e instanceof Error ? e.message : t("loot.toast.load_failed");
      setError(msg);
      toast.error(msg);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    const ac = new AbortController();
    void loadLoot(ac.signal);
    return () => ac.abort();
  }, [loadLoot]);

  return {
    data,
    loading,
    error,
    loadLoot: () => loadLoot(),
    reload: () => loadLoot(),
  };
}
