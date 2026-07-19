"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { PageHeader, ConfirmModal, StatusBadge, PageSpinner, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { AlertTriangle, Bell, FlaskConical, Globe, Link, Plus, Trash2, Zap } from "lucide-react";

interface Rule {
  id?: string;
  ID?: string;
  name?: string;
  Name?: string;
  event_type?: string;
  EventType?: string;
  enabled?: boolean;
  Enabled?: boolean;
  conditions?: unknown[];
  Conditions?: unknown[];
  actions?: unknown[];
  Actions?: unknown[];
}

interface Webhook {
  id?: number;
  ID?: number;
  name?: string;
  Name?: string;
  url?: string;
  URL?: string;
  enabled?: boolean;
  Enabled?: boolean;
  event_type?: string;
  EventType?: string;
  method?: string;
  Method?: string;
}

interface AlertRule {
  id?: number;
  name?: string;
  type?: string;
  threshold?: number;
  enabled?: boolean;
  description?: string;
}

interface MonitorAlert {
  id?: number;
  title?: string;
  message?: string;
  severity?: string;
  status?: string;
  source_name?: string;
  created_at?: string;
}

type WebhookType = "generic" | "slack" | "discord" | "email";

interface WebhookActionParams {
  type: WebhookType;
  url: string;
  secret: string;
  to: string;
  smtp_host: string;
  smtp_port: number;
  smtp_user: string;
  smtp_pass: string;
  from: string;
}

const defaultWebhookParams: WebhookActionParams = {
  type: "generic",
  url: "",
  secret: "",
  to: "",
  smtp_host: "",
  smtp_port: 587,
  smtp_user: "",
  smtp_pass: "",
  from: "",
};

export default function AutomationPage() {
  const { t } = useI18n();
  const [rules, setRules] = useState<Rule[]>([]);
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [alertRules, setAlertRules] = useState<AlertRule[]>([]);
  const [alerts, setAlerts] = useState<MonitorAlert[]>([]);
  const [showAlertRuleModal, setShowAlertRuleModal] = useState(false);
  const [alertRuleForm, setAlertRuleForm] = useState({ name: "", type: "agent_offline", threshold: 300, description: "" });
  const [showRuleModal, setShowRuleModal] = useState(false);
  const [showWebhookModal, setShowWebhookModal] = useState(false);
  const [loading, setLoading] = useState(true);
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

  const loadData = useCallback(async () => {
    try {
      const [ruleData, whResp, alertRuleResp, alertsResp] = await Promise.all([
        api.get<{ rules?: Rule[]; data?: Rule[] }>("/api/automation/rules").catch(() => null),
        api.get<{ webhooks?: Webhook[]; data?: Webhook[] }>("/api/webhooks").catch(() => null),
        api.json<{ rules?: AlertRule[] }>("/api/monitor/alert-rules").catch(() => null),
        api.get<{ alerts?: MonitorAlert[] }>("/api/monitor/alerts").catch(() => null),
      ]);
      if (ruleData) setRules((ruleData.rules || ruleData.data || []) as Rule[]);
      if (whResp) setWebhooks((whResp.webhooks || whResp.data || []) as Webhook[]);
      if (alertRuleResp) setAlertRules((alertRuleResp.rules || []) as AlertRule[]);
      if (alertsResp) setAlerts((alertsResp.alerts || []) as MonitorAlert[]);
    } catch { toast.error("Failed to load automation rules and webhooks"); } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

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
      await api.postJson("/api/automation/rules", payload);
      setShowRuleModal(false);
      setRuleForm({ name: "", event_type: "agent.checkin", action_type: "command", action_config: "", webhook: { ...defaultWebhookParams } });
      loadData();
    } catch { toast.error("Failed to save rule"); }
  };

  const handleSaveWebhook = async () => {
    try {
      await api.postJson("/api/webhooks", webhookForm);
      setShowWebhookModal(false);
      setWebhookForm({ name: "", url: "", event_type: "agent.checkin", method: "POST" });
      loadData();
    } catch { toast.error("Failed to save webhook"); }
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
      if (d.success) { toast.success("Test notification sent!"); } else { toast.error((d.error as string) || "Test failed"); }
    } catch {
      toast.error("Test notification failed");
    } finally {
      setSendingTest(false);
    }
  };

  const handleDeleteRule = (id: string) => {
    setCfm({msg: t("automation.confirm_delete_rule"), cb: async () => {
      try {
        await api.del(`/api/automation/rules/${id}`);
        loadData();
      } catch { toast.error("Failed to delete rule"); }
    }});
  };

  const handleDeleteWebhook = (id: number) => {
    setCfm({msg: t("automation.confirm_delete_webhook"), cb: async () => {
      try {
        await api.del(`/api/webhooks/${id}`);
        loadData();
      } catch { toast.error("Failed to delete webhook"); }
    }});
  };

  const handleSaveAlertRule = async () => {
    try {
      await api.postJson("/api/monitor/alert-rules", alertRuleForm);
      setShowAlertRuleModal(false);
      setAlertRuleForm({ name: "", type: "agent_offline", threshold: 300, description: "" });
      loadData();
    } catch { toast.error("Failed to save alert rule"); }
  };

  const handleDeleteAlertRule = (id: number) => {
    setCfm({ msg: "Delete this alert rule?", cb: async () => {
      try {
        await api.del(`/api/monitor/alert-rules/${id}`);
        loadData();
      } catch { toast.error("Failed to delete alert rule"); }
    }});
  };

  const handleToggleAlertRule = async (rule: AlertRule) => {
    if (!rule.id) return;
    try {
      await api.putJson(`/api/monitor/alert-rules/${rule.id}`, { enabled: !rule.enabled, name: rule.name, threshold: rule.threshold, description: rule.description });
      loadData();
    } catch { toast.error("Failed to toggle alert rule"); }
  };

  const handleAckAlert = async (id: number) => {
    try {
      await api.post(`/api/monitor/alerts/${id}/acknowledge`);
      loadData();
    } catch { toast.error("Failed to acknowledge alert"); }
  };

  const handleResolveAlert = async (id: number) => {
    try {
      await api.post(`/api/monitor/alerts/${id}/resolve`);
      loadData();
    } catch { toast.error("Failed to resolve alert"); }
  };

  const handleToggleRule = async (id: string) => {
    try {
      await api.post(`/api/automation/rules/${id}/toggle`);
      loadData();
    } catch { toast.error("Failed to toggle rule"); }
  };

  const events = [
    { type: "agent.checkin", desc: "Agent Online", color: "bg-emerald-500" },
    { type: "agent.disconnect", desc: "Agent Offline", color: "bg-red-500" },
    { type: "task.complete", desc: "Task Completed", color: "bg-blue-500" },
    { type: "task.fail", desc: "Task Failed", color: "bg-rose-500" },
    { type: "credential.found", desc: "Credential Found", color: "bg-purple-500" },
  ];

  if (loading)
    return (
      <PageSpinner />
    );

  return (
    <>

      <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title="Event System &amp; Automation" subtitle="Event-driven task chains, Webhooks, conditional triggers" />

      <div className="flex gap-4 flex-wrap">
        <Card className="rounded-xl overflow-hidden flex-1 min-w-[300px]">
          <div className="px-4 py-3 border-b border-border flex items-center gap-3">
            <div className="w-8 h-8 bg-warning/10 text-warning"><Zap className="w-4 h-4" /></div>
            <div>
              <h2 className="text-sm font-semibold text-foreground">Event Listeners</h2>
              <p className="text-xs text-muted-foreground">Built-in events: agent.checkin / disconnect / task.complete / task.fail / credential.found</p>
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

        <Card className="rounded-xl overflow-hidden flex-1 min-w-[300px]">
          <div className="px-4 py-3 border-b border-border flex items-center gap-3">
            <div className="w-8 h-8 bg-success/10 rounded-xl flex items-center justify-center text-success"><Link className="w-4 h-4" /></div>
            <div>
              <h2 className="text-sm font-semibold text-foreground">Task Chain Rules</h2>
              <p className="text-xs text-muted-foreground">Event + Condition &gt; Auto Action</p>
            </div>
          </div>
          <div className="p-4 sm:p-5">
            <div className="flex items-center justify-between mb-4">
              <span className="text-xs text-muted-foreground">{rules.length} rules</span>
              <Button onClick={() => setShowRuleModal(true)} size="sm">
                <Plus className="w-4 h-4" /> New Rule
              </Button>
            </div>
            <div className="space-y-2">
              {rules.length === 0 ? (
                <div className="text-center text-xs text-muted-foreground py-8">No rules yet. Click New Rule to create</div>
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
                          <div className="text-xs text-muted-foreground">{eventType} {conditionsLen} conditions {actionsLen} actions</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button onClick={() => handleToggleRule(rid)} variant="ghost" size="sm">
                          {enabled ? "Disable" : "Enable"}
                        </Button>
                        <Button onClick={() => handleDeleteRule(rid)} variant="destructive" size="icon-sm" aria-label="Delete rule">
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

      <Card className="rounded-xl overflow-hidden">
        <div className="px-4 py-3 border-b border-border flex items-center gap-3">
          <div className="w-8 h-8 bg-info/10 rounded-xl flex items-center justify-center text-info"><Globe className="w-4 h-4" /></div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">Webhooks</h2>
            <p className="text-xs text-muted-foreground">Event-triggered HTTP callbacks</p>
          </div>
        </div>
        <div className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4">
            <span className="text-xs text-muted-foreground">{webhooks.length} webhooks</span>
            <Button onClick={() => setShowWebhookModal(true)} size="sm">
              <Plus className="w-4 h-4" /> New Webhook
            </Button>
          </div>
          <div className="space-y-2">
            {webhooks.length === 0 ? (
              <div className="text-center text-xs text-muted-foreground py-8">No webhooks yet</div>
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
                            if (d.success) { toast.success("Webhook test sent"); } else { toast.error((d.error as string) || "Test failed"); }
                          } catch { toast.error("Webhook test failed"); }
                        }}
                        variant="ghost"
                        size="icon-sm"
                        title="Test webhook"
                        aria-label="Test webhook"
                      >
                        <FlaskConical className="w-4 h-4" />
                      </Button>
                      <Button onClick={() => handleDeleteWebhook(Number(wid))} variant="destructive" size="icon-sm" aria-label="Delete webhook">
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

      <Dialog open={showRuleModal} onOpenChange={setShowRuleModal}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>New Automation Rule</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>Rule Name</Label>
              <Input aria-label="e.g. Auto deploy on checkin" name="input-0" value={ruleForm.name} onChange={(e) => setRuleForm({ ...ruleForm, name: e.target.value })} placeholder="e.g. Auto deploy on checkin" className="mt-1" />
            </div>
            <div>
              <Label>Event Type</Label>
              <Select value={ruleForm.event_type} onValueChange={(v) => setRuleForm({ ...ruleForm, event_type: v ?? "" })}>
                <SelectTrigger className="w-full mt-1" aria-label="Event type" name="select-1"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="agent.checkin">agent.checkin</SelectItem>
                  <SelectItem value="agent.disconnect">agent.disconnect</SelectItem>
                  <SelectItem value="task.complete">task.complete</SelectItem>
                  <SelectItem value="task.fail">task.fail</SelectItem>
                  <SelectItem value="credential.found">credential.found</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label>Action Type</Label>
              <Select value={ruleForm.action_type} onValueChange={(v) => setRuleForm({ ...ruleForm, action_type: v ?? "" })}>
                <SelectTrigger className="w-full mt-1" aria-label="Action type" name="select-2"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="command">Send Command</SelectItem>
                  <SelectItem value="webhook">Send Webhook / Notification</SelectItem>
                  <SelectItem value="notify">Show Alert</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {ruleForm.action_type === "command" && (
              <div>
                <Label>Command</Label>
                <Input aria-label="e.g. whoami" name="e-g-whoami-3" placeholder="e.g. whoami" value={ruleForm.action_config} onChange={(e) => setRuleForm({ ...ruleForm, action_config: e.target.value })} className="mt-1" />
              </div>
            )}

            {ruleForm.action_type === "notify" && (
              <div>
                <Label>Notification Message</Label>
                <Input aria-label="Alert message text" name="alert-message-text-4" placeholder="Alert message text" value={ruleForm.action_config} onChange={(e) => setRuleForm({ ...ruleForm, action_config: e.target.value })} className="mt-1" />
              </div>
            )}

            {ruleForm.action_type === "webhook" && (
              <div className="space-y-4">
                <div>
                  <Label>Webhook Type</Label>
                  <Select value={ruleForm.webhook.type} onValueChange={(v) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, type: v as WebhookType } })}>
                    <SelectTrigger className="w-full mt-1" aria-label="Webhook type" name="select-5"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="generic">Generic Webhook</SelectItem>
                      <SelectItem value="slack">Slack</SelectItem>
                      <SelectItem value="discord">Discord</SelectItem>
                      <SelectItem value="email">Email (SMTP)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                {ruleForm.webhook.type !== "email" && (
                  <>
                    <div>
                      <Label>Webhook URL</Label>
                      <Input aria-label="https://hooks.example.com/..." name="https-hooks-example-com-6" placeholder="https://hooks.example.com/..." value={ruleForm.webhook.url} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, url: e.target.value } })} className="mt-1" />
                    </div>
                    <div>
                      <Label>Secret / HMAC Key (optional)</Label>
                      <Input aria-label="HMAC signing key" name="hmac-signing-key-7" placeholder="HMAC signing key" value={ruleForm.webhook.secret} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, secret: e.target.value } })} className="mt-1" />
                    </div>
                  </>
                )}

                {ruleForm.webhook.type === "email" && (
                  <>
                    <div>
                      <Label>SMTP Server</Label>
                      <Input aria-label="smtp.gmail.com" name="smtp-gmail-com-8" placeholder="smtp.gmail.com" value={ruleForm.webhook.smtp_host} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, smtp_host: e.target.value } })} className="mt-1" />
                    </div>
                    <div className="flex gap-2">
                      <div className="flex-1">
                        <Label>SMTP Port</Label>
                        <Input aria-label="587" name="587-9" type="number" placeholder="587" value={ruleForm.webhook.smtp_port} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, smtp_port: parseInt(e.target.value) || 587 } })} className="mt-1" />
                      </div>
                      <div className="flex-1">
                        <Label>From Address</Label>
                        <Input aria-label="alerts@example.com" name="alerts-example-com-10" placeholder="alerts@example.com" value={ruleForm.webhook.from} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, from: e.target.value } })} className="mt-1" />
                      </div>
                    </div>
                    <div>
                      <Label>SMTP Username</Label>
                      <Input aria-label="user@gmail.com" name="user-gmail-com-11" placeholder="user@gmail.com" value={ruleForm.webhook.smtp_user} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, smtp_user: e.target.value } })} className="mt-1" />
                    </div>
                    <div>
                      <Label>SMTP Password</Label>
                      <Input aria-label="App password" name="app-password-12" type="password" placeholder="App password" value={ruleForm.webhook.smtp_pass} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, smtp_pass: e.target.value } })} className="mt-1" />
                    </div>
                    <div>
                      <Label>To Address(es)</Label>
                      <Input aria-label="admin@example.com" name="admin-example-com-13" placeholder="admin@example.com" value={ruleForm.webhook.to} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, to: e.target.value } })} className="mt-1" />
                    </div>
                  </>
                )}

                <div className="flex justify-end">
                  <Button
                    onClick={handleTestWebhookAction}
                    disabled={sendingTest}
                    variant="ghost"
                    size="sm"
                  >
                    {sendingTest ? <Spinner size="xs" /> : <FlaskConical className="w-4 h-4" />}
                    {sendingTest ? "Sending..." : "Test Notification"}
                  </Button>
                </div>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button onClick={() => setShowRuleModal(false)} variant="ghost">Cancel</Button>
            <Button onClick={handleSaveRule}>Save Rule</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={showWebhookModal} onOpenChange={setShowWebhookModal}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>New Webhook</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>Name</Label>
              <Input aria-label="e.g. Slack alert" name="input-14" value={webhookForm.name} onChange={(e) => setWebhookForm({ ...webhookForm, name: e.target.value })} placeholder="e.g. Slack alert" className="mt-1" />
            </div>
            <div>
              <Label>URL</Label>
              <Input aria-label="https://hooks.example.com/forgec2" name="https-hooks-example-com-forgec2-15" placeholder="https://hooks.example.com/forgec2" value={webhookForm.url} onChange={(e) => setWebhookForm({ ...webhookForm, url: e.target.value })} className="mt-1" />
            </div>
            <div>
              <Label>Event Type</Label>
              <Select value={webhookForm.event_type} onValueChange={(v) => setWebhookForm({ ...webhookForm, event_type: v ?? "" })}>
                <SelectTrigger className="w-full mt-1" aria-label="Webhook event type" name="select-16"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="agent.checkin">agent.checkin</SelectItem>
                  <SelectItem value="agent.disconnect">agent.disconnect</SelectItem>
                  <SelectItem value="task.complete">task.complete</SelectItem>
                  <SelectItem value="task.fail">task.fail</SelectItem>
                  <SelectItem value="credential.found">credential.found</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label>Request Method</Label>
              <Select value={webhookForm.method} onValueChange={(v) => setWebhookForm({ ...webhookForm, method: v ?? "" })}>
                <SelectTrigger className="w-full mt-1" aria-label="Request method" name="select-17"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="POST">POST</SelectItem>
                  <SelectItem value="PUT">PUT</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => setShowWebhookModal(false)} variant="ghost">Cancel</Button>
            <Button onClick={handleSaveWebhook}>Save</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 sm:p-5">
        <Card className="rounded-xl overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 bg-destructive/10 rounded-xl flex items-center justify-center text-destructive"><Bell className="w-4 h-4" /></div>
              <div>
                <h2 className="text-sm font-semibold text-foreground">Alert Rules</h2>
                <p className="text-xs text-muted-foreground">Monitor thresholds and triggers</p>
              </div>
            </div>
            <Button onClick={() => setShowAlertRuleModal(true)} size="sm">
              <Plus className="w-4 h-4" />New
            </Button>
          </div>
          <div className="p-4 sm:p-5 space-y-2">
            {alertRules.length === 0 ? (
              <p className="text-xs text-muted-foreground text-center py-6">No alert rules configured</p>
            ) : alertRules.map((r) => (
              <div key={r.id} className="flex items-center justify-between p-3 bg-secondary border border-border rounded-xl">
                <div>
                  <div className="text-sm font-medium text-foreground">{r.name}</div>
                  <div className="text-xs text-muted-foreground">{r.type} {r.threshold}s</div>
                </div>
                <div className="flex gap-2">
                  <Button onClick={() => handleToggleAlertRule(r)} variant="ghost" size="sm">
                    {r.enabled ? "On" : "Off"}
                  </Button>
                  <Button onClick={() => r.id && handleDeleteAlertRule(r.id)} variant="destructive" size="icon-sm" aria-label="Delete"><Trash2 className="w-4 h-4" /></Button>
                </div>
              </div>
            ))}
          </div>
        </Card>

        <Card className="rounded-xl overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center gap-3">
            <div className="w-8 h-8 bg-warning/10 rounded-xl flex items-center justify-center text-warning"><AlertTriangle className="w-4 h-4" /></div>
            <div>
              <h2 className="text-sm font-semibold text-foreground">Active Alerts</h2>
              <p className="text-xs text-muted-foreground">Acknowledge or resolve incidents</p>
            </div>
          </div>
          <div className="p-4 sm:p-5 space-y-2 max-h-80 overflow-y-auto">
            {alerts.length === 0 ? (
              <p className="text-xs text-muted-foreground text-center py-6">No alerts</p>
            ) : alerts.map((a) => (
              <div key={a.id} className="p-3 bg-secondary border border-border rounded-xl">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-foreground truncate">{a.title || "Alert"}</div>
                    <div className="text-xs text-muted-foreground truncate">{a.message}</div>
                    <div className="text-xs text-muted-foreground mt-1">{a.severity} {a.status} {a.source_name || ""}</div>
                  </div>
                  {a.status === "active" && a.id != null && (
                    <div className="flex gap-1 shrink-0">
                      <Button onClick={() => handleAckAlert(Number(a.id))} variant="ghost" size="sm">Ack</Button>
                      <Button onClick={() => handleResolveAlert(Number(a.id))} variant="ghost" size="sm">Resolve</Button>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </Card>
      </div>

      <Dialog open={showAlertRuleModal} onOpenChange={setShowAlertRuleModal}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>New Alert Rule</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>Name</Label>
              <Input aria-label="Alert rule name" name="input-18" value={alertRuleForm.name} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, name: e.target.value })} className="mt-1" />
            </div>
            <div>
              <Label>Type</Label>
              <Select value={alertRuleForm.type} onValueChange={(v) => setAlertRuleForm({ ...alertRuleForm, type: v ?? "" })}>
                <SelectTrigger className="w-full mt-1" aria-label="Alert rule type" name="select-19"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="agent_offline">Agent Offline</SelectItem>
                  <SelectItem value="agent_online">Agent Online</SelectItem>
                  <SelectItem value="cpu_high">CPU High</SelectItem>
                  <SelectItem value="memory_high">Memory High</SelectItem>
                  <SelectItem value="credential_found">Credential Found</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label>Threshold</Label>
              <Input aria-label="Threshold in seconds" name="input-20" type="number" value={alertRuleForm.threshold} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, threshold: Number(e.target.value) })} className="mt-1" />
            </div>
            <div>
              <Label>Description</Label>
              <Input aria-label="Alert rule description" name="input-21" value={alertRuleForm.description} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, description: e.target.value })} className="mt-1" />
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => setShowAlertRuleModal(false)} variant="ghost">Cancel</Button>
            <Button onClick={handleSaveAlertRule}>Save</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmModal open={!!cfm} title={t("automation.confirm")} message={cfm?.msg || ""} confirmText={t("automation.delete")} cancelText={t("automation.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
    </>
  );
}

