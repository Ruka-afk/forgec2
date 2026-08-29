"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

interface SavedView {
  id: number;
  page: string;
  name: string;
  state: string;
}

/**
 * useSavedViews — per-user named filter snapshots for a list page.
 * `page` must be one of the server-side whitelist (agents, tasks, ...).
 */
export function useSavedViews(page: string) {
  const [views, setViews] = useState<SavedView[]>([]);
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(async () => {
    try {
      const d = await api.get<{ views?: SavedView[] }>(paths.settings.savedViews(page));
      setViews(d.views || []);
    } catch {
      setViews([]);
    } finally {
      setLoaded(true);
    }
  }, [page]);

  useEffect(() => { void load(); }, [load]);

  const save = useCallback(async (name: string, state: unknown) => {
    await api.postJson(paths.settings.savedViews(), {
      page,
      name,
      state: JSON.stringify(state),
    });
    await load();
  }, [page, load]);

  const remove = useCallback(async (id: number) => {
    await api.del(paths.settings.savedView(id));
    setViews((prev) => prev.filter((v) => v.id !== id));
  }, []);

  return { views, loaded, save, remove, reload: load };
}
