"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { firstArray, normalizeListEnvelope } from "@/lib/envelope";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { useWS } from "@/lib/wsContext";
import { useI18n } from "@/lib/i18n";
import type { CreateListenerForm, EditListenerForm, Listener } from "./types";
import { emptyCreateForm, emptyEditForm } from "./types";
import { indexListenerHealth, type ListenerHealth } from "./listener-health";

export function useListenersData() {
  const { t } = useI18n();
  const { subscribe } = useWS();
  const [agentCountMap, setAgentCountMap] = useState<Record<string, number>>({});
  const [creating, setCreating] = useState(false);

  const { data: listenersData, loading, error, setError, refresh: loadListeners } = useApiResource<Listener[]>({
    fetcher: async () => {
      const data = await api.get(paths.listeners.list);
      void loadHealthRef.current();
      return normalizeListEnvelope(data, ["data", "listeners", "Listeners"]) as Listener[];
    },
    retainOnError: false,
    toastThrottleMs: 10_000,
    errorMessage: t("listeners.toast_load_failed"),
  });
  const listeners = listenersData ?? [];

  const { data: healthData, refresh: loadHealth } = useApiResource<ListenerHealth[]>({
    fetcher: async (signal) => {
      const data = await api.get(paths.circuitBreaker.detail, { signal });
      return firstArray(data, ["listeners", "data"]) as ListenerHealth[];
    },
    pollMs: 15_000,
    errorMessage: "",
  });
  const loadHealthRef = useRef(loadHealth);
  loadHealthRef.current = loadHealth;

  const healthByTarget = useMemo(
    () => indexListenerHealth(healthData ?? []),
    [healthData],
  );

  const resetHealth = useCallback(
    async (listenerId: string) => {
      if (!listenerId) return;
      try {
        await api.post(paths.circuitBreaker.reset(listenerId));
        toast.success(t("listeners.reset_health_ok"));
        await loadHealth();
      } catch {
        toast.error(t("listeners.reset_health_failed"));
      }
    },
    [loadHealth, t],
  );

  const createListener = useCallback(
    async (form: CreateListenerForm) => {
      if (!form.name || !form.host || !form.port) {
        toast.error(t("listeners.toast_name_host_port_required"));
        return false;
      }
      setCreating(true);
      try {
        const data = await api.postJson(paths.listeners.list, {
          name: form.name,
          type: form.type,
          host: form.host,
          port: parseInt(form.port, 10) || 0,
          scheme: form.protocol,
          tags: form.tags,
          color: form.color,
        });
        if (data.success) {
          await loadListeners();
          toast.success(t("listeners.toast_created"));
          return true;
        }
        toast.error((data.error as string) || t("listeners.toast_unknown_error"));
        return false;
      } catch {
        toast.error(t("listeners.toast_create_failed"));
        return false;
      } finally {
        setCreating(false);
      }
    },
    [loadListeners, t],
  );

  const updateListener = useCallback(
    async (id: string, form: EditListenerForm) => {
      if (!id || !form.name || !form.host || !form.port) {
        toast.error(t("listeners.toast_name_host_port_required"));
        return false;
      }
      try {
        await api.putJson(paths.listeners.one(id), {
          name: form.name,
          type: form.type,
          host: form.host,
          port: parseInt(form.port, 10) || 0,
          protocol: form.protocol,
          notes: form.notes,
          tags: form.tags,
          color: form.color,
        });
        await loadListeners();
        toast.success(t("listeners.toast_updated"));
        return true;
      } catch {
        toast.error(t("listeners.toast_update_failed"));
        return false;
      }
    },
    [loadListeners, t],
  );

  const toggleListener = useCallback(
    async (listener: Listener) => {
      const id = listener.id || "";
      if (!id) return;
      const enabled = !(listener.enabled === true);
      try {
        if (enabled) {
          await api.postJson(paths.listeners.enable(id), {});
        } else {
          await api.postJson(paths.listeners.disable(id), {});
        }
        await loadListeners();
        toast.success(enabled ? t("listeners.toast_enabled") : t("listeners.toast_disabled"));
      } catch {
        toast.error(t("listeners.toast_toggle_failed"));
      }
    },
    [loadListeners, t],
  );

  const deleteListener = useCallback(
    async (id: string) => {
      if (!id) return;
      try {
        await api.del(paths.listeners.one(id));
        await loadListeners();
        toast.success(t("listeners.toast_deleted"));
      } catch {
        toast.error(t("listeners.toast_delete_failed"));
      }
    },
    [loadListeners, t],
  );

  useEffect(() => {
    const controller = new AbortController();
    api
      .get(paths.agents.list("page=1&pageSize=500"), { signal: controller.signal })
      .then((d) => {
        const agents = normalizeListEnvelope(d, ["agents", "Agents", "data"]);
        const map: Record<string, number> = {};
        (agents as { listener_id?: number; ListenerID?: number }[]).forEach((a) => {
          const lid = String(a.listener_id ?? a.ListenerID ?? "");
          if (lid && lid !== "0") map[lid] = (map[lid] || 0) + 1;
        });
        setAgentCountMap(map);
      })
      .catch(() => setAgentCountMap({}));
    return () => controller.abort();
  }, []);

  // Live registry updates: refresh when another operator changes a listener
  // or when the WS reconnects (sync snapshot).
  useEffect(() => {
    const unsub = subscribe((msg) => {
      if (msg.type === "listener_update" || msg.type === "sync") {
        loadListeners();
      }
    });
    return unsub;
  }, [subscribe, loadListeners]);

  return {
    listeners,
    loading,
    error,
    setError,
    agentCountMap,
    healthByTarget,
    creating,
    loadListeners,
    resetHealth,
    createListener,
    updateListener,
    toggleListener,
    deleteListener,
    emptyCreateForm,
    emptyEditForm,
  };
}
