"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { emptyCredentialData, type CredentialData, type VaultEntry } from "./types";

export function useCredentialsData() {
  const { t } = useI18n();
  const [data, setData] = useState<CredentialData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadData = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.get<{
        VaultEntries?: VaultEntry[];
        vault_entries?: VaultEntry[];
        VaultCount?: number;
        vault_count?: number;
        AllTags?: string[];
        all_tags?: string[];
      }>("/credentials?format=json", { signal });
      setData({
        VaultEntries: result.vault_entries || result.VaultEntries || [],
        VaultCount: result.vault_count ?? result.VaultCount ?? 0,
        AllTags: result.all_tags || result.AllTags || [],
      });
    } catch (e) {
      if (signal?.aborted) return;
      setData(emptyCredentialData());
      const msg = e instanceof Error ? e.message : t("cred.toast.load_failed");
      setError(msg);
      toast.error(msg);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    const controller = new AbortController();
    loadData(controller.signal);
    return () => controller.abort();
  }, [loadData]);

  return { data, setData, loading, error, loadData, reload: () => loadData() };
}
