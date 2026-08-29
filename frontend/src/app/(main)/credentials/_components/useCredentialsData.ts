"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useWS } from "@/lib/wsContext";
import { useI18n } from "@/lib/i18n";
import { paths } from "@/lib/api-paths";
import { emptyCredentialData, normalizeCredentialData, type CredentialData } from "./types";

export function useCredentialsData() {
  const { t } = useI18n();
  const { subscribe } = useWS();
  const [data, setData] = useState<CredentialData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Shared in-flight promise: two rapid WS credential_update events used to
  // both pass the loadingRef guard (only synced at render commit) and start
  // overlapping loadData()s that could resolve out of order — an older
  // response overwriting fresher vault data. Callers now join the same
  // in-flight fetch instead.
  const inflightRef = useRef<Promise<void> | null>(null);

  const loadData = useCallback(async (signal?: AbortSignal) => {
    if (!signal && inflightRef.current) return inflightRef.current;
    const run = (async () => {
      if (!signal) setLoading(true);
      setError(null);
      try {
        const result = await api.get(paths.credentials.list("format=json"), { signal });
        setData(normalizeCredentialData(result as Parameters<typeof normalizeCredentialData>[0]));
      } catch (e) {
        if (signal?.aborted) return;
        setData(emptyCredentialData());
        const msg = e instanceof Error ? e.message : t("cred.toast.load_failed");
        setError(msg);
        toast.error(msg);
      } finally {
        if (!signal?.aborted) setLoading(false);
      }
    })();
    if (!signal) {
      inflightRef.current = run.finally(() => {
        if (inflightRef.current === run) inflightRef.current = null;
      });
    }
    return run;
  }, [t]);

  useEffect(() => {
    const controller = new AbortController();
    loadData(controller.signal);
    return () => controller.abort();
  }, [loadData]);

  // Live vault updates: new credentials found by agents or edited by other
  // operators trigger a refresh; the sync snapshot covers WS reconnects.
  useEffect(() => {
    const unsub = subscribe((msg) => {
      if (msg.type === "credential_update" || msg.type === "sync") {
        void loadData();
      }
    });
    return unsub;
  }, [subscribe, loadData]);

  return { data, setData, loading, error, loadData, reload: () => loadData() };
}
