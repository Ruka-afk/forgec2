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
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError(null);
    try {
      const d = await api.get<{ views?: SavedView[] }>(paths.settings.savedViews(page));
      setViews(d.views || []);
    } catch (err) {
      // Keep any views already known. Replacing them with [] was worse than a
      // missing list: the picker stayed fully usable, and saving under an
      // existing name silently replaced that view server-side.
      setError(err instanceof Error ? err.message : String(err));
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

  return { views, loaded, error, save, remove, reload: load };
}
