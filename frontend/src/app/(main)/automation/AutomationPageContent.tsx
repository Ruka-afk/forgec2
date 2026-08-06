"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { PageHeader, ConfirmModal, EmptyState, StatusBadge, PageSpinner } from "@/components/UI";
import { DataError } from "@/components/ui/data-state";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { AlertTriangle, Bell, FlaskConical, Globe, Link, Plus, Trash2, Zap } from "lucide-react";
import { defaultWebhookParams, type AlertRule } from "./_components/types";
import { useAutomationData } from "./_components/useAutomationData";
import { RuleDialog } from "./_components/RuleDialog";
import { WebhookDialog } from "./_components/WebhookDialog";
import { AlertRuleDialog } from "./_components/AlertRuleDialog";

export default function AutomationPage() {
  const { t } = useI18n();
  const {
    rules,
    webhooks,
    alertRules,
    alerts,
    loading, error, loadData,
  } = useAutomationData();
  const [showAlertRuleModal, setShowAlertRuleModal] = useState(false);
  const [alertRuleForm, setAlertRuleForm] = useState({ name: "", type: "agent_offline", threshold: 300, description: "" });
  const [showRuleModal, setShowRuleModal] = useState(false);
  const [showWebhookModal, setShowWebhookModal] = useState(false);
  const [sendingTest, setSendingTest] = useState(false);

  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [ruleForm, setRuleForm] = useState({
    name: "",
    event_type: "agent.checkin",
    action_type: "command",
    action_config: "",
    webhook: { ...defaultWebhookParams },
  });
  const [webhookForm, setWebhookForm] = useState({ name: "", url: "", event_type: "agent.checkin", method: "POST" });

  const buildAction = () => {
    if (ruleForm.action_type === "webhook") {
      const wh = ruleForm.webhook;
      const params: Record<string, unknown> = {
        type: wh.type,
        url: wh.url,
        secret: wh.secret,
      };
      if (wh.type === "email") {
        params.to = wh.to;
        params.smtp_host = wh.smtp_host;
        params.smtp_port = wh.smtp_port;
        params.smtp_user = wh.smtp_user;
        params.smtp_pass = wh.smtp_pass;
        params.from = wh.from;
      }
      return { type: "webhook", params };
    }
    if (ruleForm.action_type === "command") {
      return { type: "command", params: { command: ruleForm.action_config } };
    }
    if (ruleForm.action_type === "notify") {
      return { type: "notify", params: { message: ruleForm.action_config, channel: "all" } };
    }
    return { type: ruleForm.action_type, params: {} };
  };

  const handleSaveRule = async () => {
    try {
      const payload = {
        name: ruleForm.name,
        event_type: ruleForm.event_type,
        actions: [buildAction()],
        conditions: [] as unknown[],
        enabled: true,
      };
      await api.postJson(paths.automation.rules, payload);
      setShowRuleModal(false);
      setRuleForm({ name: "", event_type: "agent.checkin", action_type: "command", action_config: "", webhook: { ...defaultWebhookParams } });
      loadData();
    } catch { toast.error(t("automation.toast.save_rule_failed")); }
  };

  const handleSaveWebhook = async () => {
    try {
      await api.postJson(paths.automation.webhooks, webhookForm);
      setShowWebhookModal(false);
      setWebhookForm({ name: "", url: "", event_type: "agent.checkin", method: "POST" });
      loadData();
    } catch { toast.error(t("automation.toast.save_webhook_failed")); }
  };

  const handleTestWebhookAction = async () => {
    const wh = ruleForm.webhook;
    setSendingTest(true);
    try {
      const d = await api.postJson<{ success?: boolean; error?: string }>("/api/webhooks/test", {
        type: wh.type,
        url: wh.url,
        secret: wh.secret,
        to: wh.to,
        smtp_host: wh.smtp_host,
        smtp_port: wh.smtp_port,
        smtp_user: wh.smtp_user,
        smtp_pass: wh.smtp_pass,
        from: wh.from,
      });
      if (d.success) { toast.success(t("automation.toast.test_sent")); } else { toast.error((d.error as string) || t("automation.toast.test_error")); }
    } catch {
      toast.error(t("automation.toast.test_failed"));
    } finally {
      setSendingTest(false);
    }
  };

  const handleDeleteRule = (id: string) => {
    setCfm({msg: t("automation.confirm_delete_rule"), cb: async () => {
      try {
        await api.del(paths.automation.rule(id));
        loadData();
      } catch { toast.error(t("automation.toast.delete_rule_failed")); }
    }});
  };

  const handleDeleteWebhook = (id: number) => {
    setCfm({msg: t("automation.confirm_delete_webhook"), cb: async () => {
      try {
        await api.del(paths.automation.webhook(id));
        loadData();
      } catch { toast.error(t("automation.toast.delete_webhook_failed")); }
    }});
  };

  const handleSaveAlertRule = async () => {
    try {
      await api.postJson(paths.automation.alertRules, alertRuleForm);
      setShowAlertRuleModal(false);
      setAlertRuleForm({ name: "", type: "agent_offline", threshold: 300, description: "" });
      loadData();
    } catch { toast.error(t("automation.toast.save_alert_rule_failed")); }
  };

  const handleDeleteAlertRule = (id: number) => {
    setCfm({ msg: t("automation.confirm_delete_alert_rule"), cb: async () => {
      try {
        await api.del(paths.automation.alertRule(id));
        loadData();
      } catch { toast.error(t("automation.toast.delete_alert_rule_failed")); }
    }});
  };

  const handleToggleAlertRule = async (rule: AlertRule) => {
    if (!rule.id) return;
    try {
      await api.putJson(paths.automation.alertRule(rule.id), { enabled: !rule.enabled, name: rule.name, threshold: rule.threshold, description: rule.description });
      loadData();
    } catch { toast.error(t("automation.toast.toggle_alert_rule_failed")); }
  };

  const handleAckAlert = async (id: number) => {
    try {
      await api.post(paths.automation.alertAck(id));
      loadData();
    } catch { toast.error(t("automation.toast.acknowledge_alert_failed")); }
  };

  const handleResolveAlert = async (id: number) => {
    try {
      await api.post(paths.automation.alertResolve(id));
      loadData();
    } catch { toast.error(t("automation.toast.resolve_alert_failed")); }
  };

  const handleToggleRule = async (id: string) => {
    try {
      await api.post(paths.automation.ruleToggle(id));
      loadData();
    } catch { toast.error(t("automation.toast.toggle_rule_failed")); }
  };

  const events = [
    { type: "agent.checkin", desc: t("auto.event_agent_online"), color: "bg-emerald-500" },
    { type: "agent.disconnect", desc: t("auto.event_agent_offline"), color: "bg-red-500" },
    { type: "task.complete", desc: t("auto.event_task_complete"), color: "bg-blue-500" },
    { type: "task.fail", desc: t("auto.event_task_fail"), color: "bg-rose-500" },
    { type: "credential.found", desc: t("auto.event_credential_found"), color: "bg-purple-500" },
  ];

  if (loading)
    return (
      <PageSpinner />
    );

  if (error && rules.length === 0 && webhooks.length === 0) {
    return (
      <div className="max-w-(--content-width) mx-auto pb-12">
        <PageHeader title={t("auto.title")} subtitle={t("auto.subtitle")} />
        <DataError message={error} onRetry={() => loadData()} />
      </div>
    );
  }

  return (
    <>

      <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("auto.title")} subtitle={t("auto.subtitle")} />

      <div className="flex gap-4 flex-wrap">
        <Card className="rounded-2xl overflow-hidden flex-1 min-w-[300px]">
          <div className="px-4 py-3 border-b border-border flex items-center gap-3">
            <div className="w-8 h-8 bg-warning/10 text-warning"><Zap className="w-4 h-4" /></div>
            <div>
              <h2 className="text-sm font-semibold text-foreground">{t("auto.event_listeners")}</h2>
              <p className="text-xs text-muted-foreground">{t("auto.event_listeners_desc")}</p>
            </div>
          </div>
          <div className="p-4 sm:p-5 space-y-3">
            {events.map((e) => (
              <div key={e.type} className="flex items-center gap-3 p-3 bg-secondary border border-border rounded-xl">
                <span className={`w-2 h-2 ${e.color} rounded-full`}></span>
                <span className="font-mono text-xs text-foreground">{e.type}</span>
                <span className="text-muted-foreground">-</span>
                <span className="text-muted-foreground">{e.desc}</span>
              </div>
            ))}
          </div>
        </Card>

        <Card className="rounded-2xl overflow-hidden flex-1 min-w-[300px]">
          <div className="px-4 py-3 border-b border-border flex items-center gap-3">
            <div className="w-8 h-8 bg-success/10 rounded-xl flex items-center justify-center text-success"><Link className="w-4 h-4" /></div>
            <div>
              <h2 className="text-sm font-semibold text-foreground">{t("auto.task_chain_rules")}</h2>
              <p className="text-xs text-muted-foreground">{t("auto.task_chain_desc")}</p>
            </div>
          </div>
          <div className="p-4 sm:p-5">
            <div className="flex items-center justify-between mb-4">
              <span className="text-xs text-muted-foreground">{t("auto.rules_count", { count: rules.length })}</span>
              <Button onClick={() => setShowRuleModal(true)} size="sm">
                <Plus className="w-4 h-4" /> {t("auto.new_rule")}
              </Button>
            </div>
            <div className="space-y-2">
              {rules.length === 0 ? (
                <div className="text-center text-xs text-muted-foreground py-8">
                  <EmptyState icon={Zap} title={t("automation.empty_rules")} message={t("automation.empty_rules_hint")} />
                </div>
              ) : (
                rules.map((r, i) => {
                  const rid = r.id || String(i);
                  const name = r.name || "-";
                  const eventType = r.event_type || "-";
                  const enabled = r.enabled !== undefined ? r.enabled : r.Enabled;
                  const conditionsLen = r.conditions ? r.conditions.length : (r.Conditions ? r.Conditions.length : 0);
                  const actionsLen = r.actions ? r.actions.length : (r.Actions ? r.Actions.length : 0);
                  return (
                    <div key={rid} className="flex items-center justify-between p-3 bg-secondary border border-border rounded-xl">
                      <div className="flex items-center gap-3">
                        <StatusBadge status={enabled ? "online" : "offline"} />
                        <div>
                          <div className="text-sm font-medium text-foreground">{name}</div>
                          <div className="text-xs text-muted-foreground">{eventType} {t("auto.rule_conditions_actions", { conditions: conditionsLen, actions: actionsLen })}</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button onClick={() => handleToggleRule(rid)} variant="ghost" size="sm">
                          {enabled ? t("auto.disable") : t("auto.enable")}
                        </Button>
                        <Button onClick={() => handleDeleteRule(rid)} variant="destructive" size="icon-sm" aria-label={t("automation.delete_rule")}>
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>
        </Card>
      </div>

      <Card className="rounded-2xl overflow-hidden">
        <div className="px-4 py-3 border-b border-border flex items-center gap-3">
          <div className="w-8 h-8 bg-info/10 rounded-xl flex items-center justify-center text-info"><Globe className="w-4 h-4" /></div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">{t("auto.webhooks")}</h2>
            <p className="text-xs text-muted-foreground">{t("auto.webhooks_desc")}</p>
          </div>
        </div>
        <div className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4">
            <span className="text-xs text-muted-foreground">{t("auto.webhooks_count", { count: webhooks.length })}</span>
            <Button onClick={() => setShowWebhookModal(true)} size="sm">
              <Plus className="w-4 h-4" /> {t("auto.new_webhook")}
            </Button>
          </div>
          <div className="space-y-2">
            {webhooks.length === 0 ? (
              <div className="text-center text-xs text-muted-foreground py-8">
                <EmptyState icon={Bell} title={t("automation.empty_webhooks")} />
              </div>
            ) : (
              webhooks.map((w, i) => {
                const wid = w.id || i;
                const name = w.name || "-";
                const url = w.url || "-";
                const eventType = w.event_type || "-";
                const enabled = w.enabled !== undefined ? w.enabled : w.Enabled;
                return (
                  <div key={String(wid)} className="flex items-center justify-between p-3 bg-secondary border border-border rounded-xl">
                    <div className="flex items-center gap-3">
                      <StatusBadge status={enabled ? "online" : "offline"} />
                      <div>
                        <div className="text-sm font-medium text-foreground">{name}</div>
                        <div className="text-xs text-muted-foreground font-mono">{url}</div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-muted-foreground">{eventType}</span>
                      <Button
                        onClick={async () => {
                          try {
                            const d = await api.postJson<{ success?: boolean; error?: string }>("/api/webhooks/test", { url, method: w.method || "POST" });
                             if (d.success) { toast.success(t("automation.toast.webhook_test_sent")); } else { toast.error((d.error as string) || t("automation.toast.webhook_test_failed")); }
                          } catch { toast.error(t("automation.toast.webhook_test_failed")); }
                        }}
                        variant="ghost"
                        size="icon-sm"
                        aria-label={t("automation.test_webhook")}
                      >
                        <Tooltip>
                          <TooltipTrigger>
                            <span><FlaskConical className="w-4 h-4" /></span>
                          </TooltipTrigger>
                          <TooltipContent>{t("auto.test_webhook")}</TooltipContent>
                        </Tooltip>
                      </Button>
                      <Button onClick={() => handleDeleteWebhook(Number(wid))} variant="destructive" size="icon-sm" aria-label={t("automation.delete_webhook")}>
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      </Card>

      <RuleDialog
        open={showRuleModal}
        onOpenChange={setShowRuleModal}
        ruleForm={ruleForm}
        setRuleForm={setRuleForm}
        sendingTest={sendingTest}
        onTestWebhook={handleTestWebhookAction}
        onSave={handleSaveRule}
      />

      <WebhookDialog
        open={showWebhookModal}
        onOpenChange={setShowWebhookModal}
        webhookForm={webhookForm}
        setWebhookForm={setWebhookForm}
        onSave={handleSaveWebhook}
      />


      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 sm:p-5">
        <Card className="rounded-2xl overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 bg-destructive/10 rounded-xl flex items-center justify-center text-destructive"><Bell className="w-4 h-4" /></div>
              <div>
                <h2 className="text-sm font-semibold text-foreground">{t("auto.alert_rules")}</h2>
                <p className="text-xs text-muted-foreground">{t("auto.alert_rules_desc")}</p>
              </div>
            </div>
            <Button onClick={() => setShowAlertRuleModal(true)} size="sm">
              <Plus className="w-4 h-4" />{t("auto.new")}
            </Button>
          </div>
          <div className="p-4 sm:p-5 space-y-2">
            {alertRules.length === 0 ? (
              <p className="text-xs text-muted-foreground text-center py-6">{t("auto.no_alert_rules")}</p>
            ) : alertRules.map((r) => (
              <div key={r.id} className="flex items-center justify-between p-3 bg-secondary border border-border rounded-xl">
                <div>
                  <div className="text-sm font-medium text-foreground">{r.name}</div>
                  <div className="text-xs text-muted-foreground">{r.type} {r.threshold}s</div>
                </div>
                <div className="flex gap-2">
                  <Button onClick={() => handleToggleAlertRule(r)} variant="ghost" size="sm">
                    {r.enabled ? t("auto.on") : t("auto.off")}
                  </Button>
                  <Button onClick={() => r.id && handleDeleteAlertRule(r.id)} variant="destructive" size="icon-sm" aria-label={t("automation.delete_alert_rule")}><Trash2 className="w-4 h-4" /></Button>
                </div>
              </div>
            ))}
          </div>
        </Card>

        <Card className="rounded-2xl overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center gap-3">
            <div className="w-8 h-8 bg-warning/10 rounded-xl flex items-center justify-center text-warning"><AlertTriangle className="w-4 h-4" /></div>
            <div>
              <h2 className="text-sm font-semibold text-foreground">{t("auto.active_alerts")}</h2>
              <p className="text-xs text-muted-foreground">{t("auto.active_alerts_desc")}</p>
            </div>
          </div>
          <div className="p-4 sm:p-5 space-y-2 max-h-80 overflow-y-auto">
            {alerts.length === 0 ? (
              <p className="text-xs text-muted-foreground text-center py-6">{t("auto.no_alerts")}</p>
            ) : alerts.map((a) => (
              <div key={a.id} className="p-3 bg-secondary border border-border rounded-xl">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-foreground truncate">{a.title || t("auto.alert")}</div>
                    <div className="text-xs text-muted-foreground truncate">{a.message}</div>
                    <div className="text-xs text-muted-foreground mt-1">{a.severity} {a.status} {a.source_name || ""}</div>
                  </div>
                  {a.status === "active" && a.id != null && (
                    <div className="flex gap-1 shrink-0">
                      <Button onClick={() => handleAckAlert(Number(a.id))} variant="ghost" size="sm">{t("auto.ack")}</Button>
                      <Button onClick={() => handleResolveAlert(Number(a.id))} variant="ghost" size="sm">{t("auto.resolve")}</Button>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </Card>
      </div>


      <AlertRuleDialog
        open={showAlertRuleModal}
        onOpenChange={setShowAlertRuleModal}
        alertRuleForm={alertRuleForm}
        setAlertRuleForm={setAlertRuleForm}
        onSave={handleSaveAlertRule}
      />


      <ConfirmModal open={!!cfm} title={t("automation.confirm")} message={cfm?.msg || ""} confirmText={t("automation.delete")} cancelText={t("automation.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
    </>
  );
}

