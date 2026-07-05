"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal } from "@/components/UI";

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


export default function AutomationPage() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [alertRules, setAlertRules] = useState<AlertRule[]>([]);
  const [alerts, setAlerts] = useState<MonitorAlert[]>([]);
  const [showAlertRuleModal, setShowAlertRuleModal] = useState(false);
  const [alertRuleForm, setAlertRuleForm] = useState({ name: "", type: "agent_offline", threshold: 300, description: "" });
  const [showRuleModal, setShowRuleModal] = useState(false);
  const [showWebhookModal, setShowWebhookModal] = useState(false);
  const [loading, setLoading] = useState(true);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [ruleForm, setRuleForm] = useState({ name: "", event_type: "agent.checkin", action_type: "command", action_config: "" });
  const [webhookForm, setWebhookForm] = useState({ name: "", url: "", event_type: "agent.checkin", method: "POST" });

  const loadData = useCallback(async () => {
    try {
      const [autoResp, rulesResp, alertsResp] = await Promise.all([
        fetch(`${API_BASE}?p=/automation&format=json`),
        fetch(`${API_BASE}?p=/api/monitor/alert-rules&format=json`),
        fetch(`${API_BASE}?p=/api/monitor/alerts&format=json`),
      ]);
      if (autoResp.ok) {
        const data = await autoResp.json();
        setRules(data.rules || data.Rules || []);
        setWebhooks(data.webhooks || data.Webhooks || []);
      }
      if (rulesResp.ok) {
        const d = await rulesResp.json();
        setAlertRules(d.rules || []);
      }
      if (alertsResp.ok) {
        const d = await alertsResp.json();
        setAlerts(d.alerts || []);
      }
    } catch (e) { console.error("Automation: load data failed", e); } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadData()); }, [loadData]);

  const handleSaveRule = async () => {
    try {
      await fetch(`${API_BASE}?p=/api/automation/rules&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(ruleForm),
      });
      setShowRuleModal(false);
      setRuleForm({ name: "", event_type: "agent.checkin", action_type: "command", action_config: "" });
      loadData();
    } catch (e) { console.error("Automation: save rule failed", e); }
  };

  const handleSaveWebhook = async () => {
    try {
      await fetch(`${API_BASE}?p=/api/webhooks&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(webhookForm),
      });
      setShowWebhookModal(false);
      setWebhookForm({ name: "", url: "", event_type: "agent.checkin", method: "POST" });
      loadData();
    } catch (e) { console.error("Automation: save webhook failed", e); }
  };

  const handleDeleteRule = (id: string) => {
    setCfm({msg: "确认删除此规则", cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/api/automation/rules/${id}&format=json`, { method: "DELETE" });
        loadData();
      } catch (e) { console.error("Automation: delete rule failed", e); }
    }});
  };

  const handleDeleteWebhook = (id: number) => {
    setCfm({msg: "确认删除Webhook", cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/api/webhooks/${id}&format=json`, { method: "DELETE" });
        loadData();
      } catch (e) { console.error("Automation: delete webhook failed", e); }
    }});
  };

  const handleSaveAlertRule = async () => {
    try {
      await fetch(`${API_BASE}?p=/api/monitor/alert-rules&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify(alertRuleForm),
      });
      setShowAlertRuleModal(false);
      setAlertRuleForm({ name: "", type: "agent_offline", threshold: 300, description: "" });
      loadData();
    } catch (e) { console.error("Automation: save alert rule failed", e); }
  };

  const handleDeleteAlertRule = (id: number) => {
    setCfm({ msg: "Delete this alert rule?", cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/api/monitor/alert-rules/${id}&format=json`, { method: "DELETE", credentials: "include" });
        loadData();
      } catch (e) { console.error("Automation: delete alert rule failed", e); }
    }});
  };

  const handleToggleAlertRule = async (rule: AlertRule) => {
    if (!rule.id) return;
    try {
      await fetch(`${API_BASE}?p=/api/monitor/alert-rules/${rule.id}&format=json`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ enabled: !rule.enabled, name: rule.name, threshold: rule.threshold, description: rule.description }),
      });
      loadData();
    } catch (e) { console.error("Automation: toggle alert rule failed", e); }
  };

  const handleAckAlert = async (id: number) => {
    try {
      await fetch(`${API_BASE}?p=/api/monitor/alerts/${id}/acknowledge&format=json`, { method: "POST", credentials: "include" });
      loadData();
    } catch (e) { console.error("Automation: ack alert failed", e); }
  };

  const handleResolveAlert = async (id: number) => {
    try {
      await fetch(`${API_BASE}?p=/api/monitor/alerts/${id}/resolve&format=json`, { method: "POST", credentials: "include" });
      loadData();
    } catch (e) { console.error("Automation: resolve alert failed", e); }
  };

  const handleToggleRule = async (id: string) => {
    try {
      await fetch(`${API_BASE}?p=/api/automation/rules/${id}/toggle&format=json`, {
        method: "POST",
      });
      loadData();
    } catch (e) { console.error("Automation: toggle rule failed", e); }
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
      <div className="flex items-center justify-center h-64">
        <i className="fa-solid fa-circle-notch fa-spin text-3xl text-indigo-500"></i>
      </div>
    );

  return (
    <div className="max-w-6xl mx-auto space-y-6 mb-20 md:mb-0">
      <div className="bg-gradient-to-r from-amber-600 to-orange-600 rounded-3xl shadow-sm p-6 text-white">
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 bg-white/10 rounded-2xl flex items-center justify-center"><i className="fa-solid fa-robot text-2xl"></i></div>
          <div>
            <h1 className="text-xl font-bold">Event System & Automation</h1>
            <p className="text-sm text-amber-200">Event-driven task chains, Webhooks, conditional triggers</p>
          </div>
        </div>
      </div>

      <div className="flex gap-4 flex-wrap">
        <div className="ui-card rounded-3xl shadow-sm overflow-hidden flex-1 min-w-[300px]">
          <div className="px-6 py-4 border-b border-slate-100 flex items-center gap-3">
            <div className="w-8 h-8 bg-amber-100 rounded-xl flex items-center justify-center text-amber-600"><i className="fa-solid fa-bolt"></i></div>
            <div>
              <h2 className="text-sm font-semibold">Event Listeners</h2>
              <p className="text-[11px] text-slate-500">Built-in events: agent.checkin / disconnect / task.complete / task.fail / credential.found</p>
            </div>
          </div>
          <div className="p-6 space-y-3 text-sm text-slate-600">
            {events.map((e) => (
              <div key={e.type} className="flex items-center gap-3 p-3 bg-slate-50 dark:bg-slate-700/50 rounded-xl">
                <span className={`w-2 h-2 ${e.color} rounded-full`}></span>
                <span className="font-mono text-xs">{e.type}</span>
                <span className="text-slate-400">-</span>
                <span>{e.desc}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="ui-card rounded-3xl shadow-sm overflow-hidden flex-1 min-w-[300px]">
          <div className="px-6 py-4 border-b border-slate-100 flex items-center gap-3">
            <div className="w-8 h-8 bg-emerald-100 rounded-xl flex items-center justify-center text-emerald-600"><i className="fa-solid fa-link"></i></div>
            <div>
              <h2 className="text-sm font-semibold">Task Chain Rules</h2>
              <p className="text-[11px] text-slate-500">Event + Condition {">"} Auto Action</p>
            </div>
          </div>
          <div className="p-6">
            <div className="flex items-center justify-between mb-4">
              <span className="text-xs text-slate-500 dark:text-slate-400">{rules.length} rules</span>
              <button onClick={() => setShowRuleModal(true)} className="px-3 py-1.5 bg-amber-600 hover:bg-amber-700 text-white rounded-xl text-xs font-medium flex items-center gap-1.5">
                <i className="fa-solid fa-plus"></i> New Rule
              </button>
            </div>
            <div className="space-y-2">
              {rules.length === 0 ? (
                <div className="text-center text-xs text-slate-400 dark:text-slate-500 py-8">No rules yet. Click New Rule to create</div>
              ) : (
                rules.map((r, i) => {
                  const rid = r.id || r.ID || String(i);
                  const name = r.name || r.Name || "-";
                  const eventType = r.event_type || r.EventType || "-";
                  const enabled = r.enabled !== undefined ? r.enabled : r.Enabled;
                  const conditionsLen = r.conditions ? r.conditions.length : (r.Conditions ? r.Conditions.length : 0);
                  const actionsLen = r.actions ? r.actions.length : (r.Actions ? r.Actions.length : 0);
                  return (
                    <div key={rid} className="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-700/50 rounded-xl">
                      <div className="flex items-center gap-3">
                        <span className={`w-2 h-2 ${enabled ? "bg-emerald-500" : "bg-slate-300"} rounded-full`}></span>
                        <div>
                          <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{name}</div>
                          <div className="text-[10px] text-slate-500 dark:text-slate-400">{eventType} {conditionsLen} conditions {actionsLen} actions</div>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <button onClick={() => handleToggleRule(rid)} className={`text-xs px-2 py-1 rounded-lg ${enabled ? "bg-amber-100 text-amber-700" : "bg-slate-200 text-slate-500 dark:text-slate-400"}`}>
                          {enabled ? "Disable" : "Enable"}
                        </button>
                        <button onClick={() => handleDeleteRule(rid)} className="text-xs px-2 py-1 bg-red-100 text-red-600 rounded-lg hover:bg-red-200">
                          <i className="fa-solid fa-trash"></i>
                        </button>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>
        </div>
      </div>

      <div className="ui-card rounded-3xl shadow-sm overflow-hidden">
        <div className="px-6 py-4 border-b border-slate-100 flex items-center gap-3">
          <div className="w-8 h-8 bg-sky-100 rounded-xl flex items-center justify-center text-sky-600"><i className="fa-solid fa-globe"></i></div>
          <div>
            <h2 className="text-sm font-semibold">Webhooks</h2>
            <p className="text-[11px] text-slate-500">Event-triggered HTTP callbacks</p>
          </div>
        </div>
        <div className="p-6">
          <div className="flex items-center justify-between mb-4">
            <span className="text-xs text-slate-500 dark:text-slate-400">{webhooks.length} webhooks</span>
            <button onClick={() => setShowWebhookModal(true)} className="px-3 py-1.5 bg-sky-600 hover:bg-sky-700 text-white rounded-xl text-xs font-medium flex items-center gap-1.5">
              <i className="fa-solid fa-plus"></i> New Webhook
            </button>
          </div>
          <div className="space-y-2">
            {webhooks.length === 0 ? (
              <div className="text-center text-xs text-slate-400 dark:text-slate-500 py-8">No webhooks yet</div>
            ) : (
              webhooks.map((w, i) => {
                const wid = w.id || w.ID || i;
                const name = w.name || w.Name || "-";
                const url = w.url || w.URL || "-";
                const eventType = w.event_type || w.EventType || "-";
                const enabled = w.enabled !== undefined ? w.enabled : w.Enabled;
                return (
                  <div key={String(wid)} className="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-700/50 rounded-xl">
                    <div className="flex items-center gap-3">
                      <span className={`w-2 h-2 ${enabled ? "bg-emerald-500" : "bg-slate-300"} rounded-full`}></span>
                      <div>
                        <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{name}</div>
                        <div className="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{url}</div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-slate-400 dark:text-slate-500">{eventType}</span>
                      <button
                        onClick={async () => {
                          try {
                            const r = await fetch(`${API_BASE}?p=/api/webhooks/test&format=json`, {
                              method: "POST",
                              credentials: "include",
                              headers: { "Content-Type": "application/json" },
                              body: JSON.stringify({ url, method: w.method || w.Method || "POST" }),
                            });
                            const d = await r.json();
                            alert(d.success ? "Webhook test sent" : (d.error || "Test failed"));
                          } catch { alert("Test failed"); }
                        }}
                        className="text-xs px-2 py-1 bg-sky-100 text-sky-600 rounded-lg hover:bg-sky-200"
                        title="Test webhook"
                      >
                        <i className="fa-solid fa-vial"></i>
                      </button>
                      <button onClick={() => handleDeleteWebhook(Number(wid))} className="text-xs px-2 py-1 bg-red-100 text-red-600 rounded-lg hover:bg-red-200">
                        <i className="fa-solid fa-trash"></i>
                      </button>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      </div>

      {showRuleModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowRuleModal(false)}>
          <div className="bg-[var(--card-bg)] rounded-3xl shadow-xl w-full max-w-lg mx-4 max-h-[85vh] overflow-auto" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border)] sticky top-0 bg-[var(--card-bg)] z-10">
              <div className="font-semibold text-slate-900 dark:text-slate-100">New Automation Rule</div>
              <button onClick={() => setShowRuleModal(false)} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 text-xl">&times;</button>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Rule Name</label>
                <input className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1 focus:outline-none focus:border-amber-500" value={ruleForm.name} onChange={(e) => setRuleForm({ ...ruleForm, name: e.target.value })} placeholder="e.g. Auto deploy on checkin" />
              </div>
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Event Type</label>
                <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1 focus:outline-none focus:border-amber-500" value={ruleForm.event_type} onChange={(e) => setRuleForm({ ...ruleForm, event_type: e.target.value })}>
                  <option value="agent.checkin">agent.checkin</option>
                  <option value="agent.disconnect">agent.disconnect</option>
                  <option value="task.complete">task.complete</option>
                  <option value="task.fail">task.fail</option>
                  <option value="credential.found">credential.found</option>
                </select>
              </div>
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Action Type</label>
                <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1 focus:outline-none focus:border-amber-500" value={ruleForm.action_type} onChange={(e) => setRuleForm({ ...ruleForm, action_type: e.target.value })}>
                  <option value="command">Send Command</option>
                  <option value="webhook">Call Webhook</option>
                  <option value="alert">Show Alert</option>
                </select>
              </div>
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Action Config</label>
                <input className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1 focus:outline-none focus:border-amber-500" placeholder="Command string, webhook URL, or alert message" value={ruleForm.action_config} onChange={(e) => setRuleForm({ ...ruleForm, action_config: e.target.value })} />
              </div>
            </div>
            <div className="px-6 py-4 border-t border-[var(--border)] bg-slate-50 dark:bg-slate-800/50 flex justify-end gap-2 rounded-b-3xl">
              <button onClick={() => setShowRuleModal(false)} className="px-4 py-2 text-sm rounded-xl bg-[var(--card-bg)] border border-[var(--border)] hover:bg-slate-100 dark:hover:bg-slate-600 text-[var(--text-secondary)]">Cancel</button>
              <button onClick={handleSaveRule} className="px-4 py-2 text-sm rounded-xl bg-amber-600 hover:bg-amber-700 text-white">Save Rule</button>
            </div>
          </div>
        </div>
      )}

      {showWebhookModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowWebhookModal(false)}>
          <div className="bg-[var(--card-bg)] rounded-3xl shadow-xl w-full max-w-md mx-4" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border)]">
              <div className="font-semibold text-slate-900 dark:text-slate-100">New Webhook</div>
              <button onClick={() => setShowWebhookModal(false)} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 text-xl">&times;</button>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Name</label>
                <input className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1 focus:outline-none focus:border-sky-500" value={webhookForm.name} onChange={(e) => setWebhookForm({ ...webhookForm, name: e.target.value })} placeholder="e.g. Slack alert" />
              </div>
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">URL</label>
                <input className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1 focus:outline-none focus:border-sky-500" placeholder="https://hooks.example.com/forgec2" value={webhookForm.url} onChange={(e) => setWebhookForm({ ...webhookForm, url: e.target.value })} />
              </div>
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Event Type</label>
                <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1 focus:outline-none focus:border-sky-500" value={webhookForm.event_type} onChange={(e) => setWebhookForm({ ...webhookForm, event_type: e.target.value })}>
                  <option value="agent.checkin">agent.checkin</option>
                  <option value="agent.disconnect">agent.disconnect</option>
                  <option value="task.complete">task.complete</option>
                  <option value="task.fail">task.fail</option>
                  <option value="credential.found">credential.found</option>
                </select>
              </div>
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Request Method</label>
                <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1 focus:outline-none focus:border-sky-500" value={webhookForm.method} onChange={(e) => setWebhookForm({ ...webhookForm, method: e.target.value })}>
                  <option>POST</option>
                  <option>PUT</option>
                </select>
              </div>
            </div>
            <div className="px-6 py-4 border-t border-[var(--border)] bg-slate-50 dark:bg-slate-800/50 flex justify-end gap-2 rounded-b-3xl">
              <button onClick={() => setShowWebhookModal(false)} className="px-4 py-2 text-sm rounded-xl bg-[var(--card-bg)] border border-[var(--border)] hover:bg-slate-100 dark:hover:bg-slate-600 text-[var(--text-secondary)]">Cancel</button>
              <button onClick={handleSaveWebhook} className="px-4 py-2 text-sm rounded-xl bg-sky-600 hover:bg-sky-700 text-white">Save</button>
            </div>
          </div>
        </div>
      )}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="ui-card rounded-3xl shadow-sm overflow-hidden">
          <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 bg-rose-100 rounded-xl flex items-center justify-center text-rose-600"><i className="fa-solid fa-bell"></i></div>
              <div>
                <h2 className="text-sm font-semibold">Alert Rules</h2>
                <p className="text-[11px] text-slate-500">Monitor thresholds and triggers</p>
              </div>
            </div>
            <button onClick={() => setShowAlertRuleModal(true)} className="px-3 py-1.5 bg-rose-600 hover:bg-rose-700 text-white rounded-xl text-xs font-medium">
              <i className="fa-solid fa-plus mr-1"></i>New
            </button>
          </div>
          <div className="p-6 space-y-2">
            {alertRules.length === 0 ? (
              <p className="text-xs text-slate-400 text-center py-6">No alert rules configured</p>
            ) : alertRules.map((r) => (
              <div key={r.id} className="flex items-center justify-between p-3 bg-slate-50 dark:bg-slate-700/50 rounded-xl">
                <div>
                  <div className="text-sm font-medium">{r.name}</div>
                  <div className="text-[10px] text-slate-500">{r.type} 路 threshold {r.threshold}</div>
                </div>
                <div className="flex gap-2">
                  <button onClick={() => handleToggleAlertRule(r)} className={`text-xs px-2 py-1 rounded-lg ${r.enabled ? "bg-emerald-100 text-emerald-700" : "bg-slate-200 text-slate-500"}`}>
                    {r.enabled ? "On" : "Off"}
                  </button>
                  <button onClick={() => r.id && handleDeleteAlertRule(r.id)} className="text-xs px-2 py-1 bg-red-100 text-red-600 rounded-lg"><i className="fa-solid fa-trash"></i></button>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="ui-card rounded-3xl shadow-sm overflow-hidden">
          <div className="px-6 py-4 border-b border-slate-100 flex items-center gap-3">
            <div className="w-8 h-8 bg-orange-100 rounded-xl flex items-center justify-center text-orange-600"><i className="fa-solid fa-triangle-exclamation"></i></div>
            <div>
              <h2 className="text-sm font-semibold">Active Alerts</h2>
              <p className="text-[11px] text-slate-500">Acknowledge or resolve incidents</p>
            </div>
          </div>
          <div className="p-6 space-y-2 max-h-80 overflow-y-auto">
            {alerts.length === 0 ? (
              <p className="text-xs text-slate-400 text-center py-6">No alerts</p>
            ) : alerts.map((a) => (
              <div key={a.id} className="p-3 bg-slate-50 dark:bg-slate-700/50 rounded-xl">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="text-sm font-medium truncate">{a.title || "Alert"}</div>
                    <div className="text-[10px] text-slate-500 truncate">{a.message}</div>
                    <div className="text-[10px] text-slate-400 mt-1">{a.severity} 路 {a.status} 路 {a.source_name || ""}</div>
                  </div>
                  {a.status === "active" && a.id && (
                    <div className="flex gap-1 shrink-0">
                      <button onClick={() => handleAckAlert(a.id!)} className="text-[10px] px-2 py-1 bg-amber-100 text-amber-700 rounded-lg">Ack</button>
                      <button onClick={() => handleResolveAlert(a.id!)} className="text-[10px] px-2 py-1 bg-emerald-100 text-emerald-700 rounded-lg">Resolve</button>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {showAlertRuleModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowAlertRuleModal(false)}>
          <div className="bg-[var(--card-bg)] rounded-3xl shadow-xl w-full max-w-md mx-4" onClick={(e) => e.stopPropagation()}>
            <div className="px-6 py-4 border-b border-[var(--border)] font-semibold">New Alert Rule</div>
            <div className="p-6 space-y-4">
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Name</label>
                <input className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1" value={alertRuleForm.name} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, name: e.target.value })} />
              </div>
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Type</label>
                <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1" value={alertRuleForm.type} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, type: e.target.value })}>
                  <option value="agent_offline">Agent Offline</option>
                  <option value="agent_online">Agent Online</option>
                  <option value="cpu_high">CPU High</option>
                  <option value="memory_high">Memory High</option>
                  <option value="credential_found">Credential Found</option>
                </select>
              </div>
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Threshold</label>
                <input type="number" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1" value={alertRuleForm.threshold} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, threshold: Number(e.target.value) })} />
              </div>
              <div>
                <label className="text-xs font-medium text-[var(--text-secondary)]">Description</label>
                <input className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm mt-1" value={alertRuleForm.description} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, description: e.target.value })} />
              </div>
            </div>
            <div className="px-6 py-4 border-t flex justify-end gap-2">
              <button onClick={() => setShowAlertRuleModal(false)} className="px-4 py-2 text-sm rounded-xl border">Cancel</button>
              <button onClick={handleSaveAlertRule} className="px-4 py-2 text-sm rounded-xl bg-rose-600 text-white">Save</button>
            </div>
          </div>
        </div>
      )}

      <ConfirmModal open={!!cfm} title="纭" message={cfm?.msg || ""} confirmText="鍒犻櫎" cancelText="取消" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
