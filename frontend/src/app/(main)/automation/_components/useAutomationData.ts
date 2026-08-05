"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import type { AlertRule, MonitorAlert, Rule, Webhook } from "./types";

export function useAutomationData() {
  const { t } = useI18n();
  const [rules, setRules] = useState<Rule[]>([]);
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [alertRules, setAlertRules] = useState<AlertRule[]>([]);
  const [alerts, setAlerts] = useState<MonitorAlert[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadData = useCallback(async (signal?: AbortSignal) => {
    setError(null);
    try {
      let failed = 0;
      const [ruleData, whResp, alertRuleResp, alertsResp] = await Promise.all([
        api.get<{ rules?: Rule[]; data?: Rule[] }>("/api/automation/rules", { signal }).catch(() => {
          failed++;
          return null;
        }),
        api.get<{ webhooks?: Webhook[]; data?: Webhook[] }>("/api/webhooks", { signal }).catch(() => {
          failed++;
          return null;
        }),
        api.get<{ rules?: AlertRule[] }>("/api/monitor/alert-rules", { signal }).catch(() => {
          failed++;
          return null;
        }),
        api.get<{ alerts?: MonitorAlert[] }>("/api/monitor/alerts", { signal }).catch(() => {
          failed++;
          return null;
        }),
      ]);
      if (signal?.aborted) return;
      if (ruleData) setRules((ruleData.rules || ruleData.data || []) as Rule[]);
      if (whResp) setWebhooks((whResp.webhooks || whResp.data || []) as Webhook[]);
      if (alertRuleResp) setAlertRules((alertRuleResp.rules || []) as AlertRule[]);
      if (alertsResp) setAlerts((alertsResp.alerts || []) as MonitorAlert[]);
      if (failed > 0) {
        const msg = t("automation.toast.load_failed");
        setError(failed === 4 ? msg : null);
        toast.error(msg);
      }
    } catch (e) {
      if (signal?.aborted) return;
      const msg = e instanceof Error ? e.message : t("common.error");
      setError(msg);
      toast.error(msg);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    const ac = new AbortController();
    void loadData(ac.signal);
    return () => ac.abort();
  }, [loadData]);

  return {
    rules,
    setRules,
    webhooks,
    setWebhooks,
    alertRules,
    setAlertRules,
    alerts,
    setAlerts,
    loading,
    error,
    loadData: () => loadData(),
    reload: () => {
      setLoading(true);
      return loadData();
    },
  };
}
