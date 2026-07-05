"use client";

import { useEffect, useState } from "react";
import { API_BASE } from "@/lib/constants";
import { PageHeader } from "@/components/UI";

interface OpsecRule {
  name: string;
  description: string;
  risk_level: number;
  default_action: number;
}

export default function OpsecPage() {
  const [rules, setRules] = useState<OpsecRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [testResult, setTestResult] = useState<{ allowed: boolean; messages: string } | null>(null);

  useEffect(() => {
    fetch(`${API_BASE}?p=/api/opsec/rules&format=json`)
      .then((r) => r.json())
      .then((data) => setRules(data.rules || []))
      .catch(() => setRules([]))
      .finally(() => setLoading(false));
  }, []);

  const riskLabel = (level: number) => {
    switch (level) {
      case 4: return <span className="text-red-600 dark:text-red-400 font-semibold">CRITICAL</span>;
      case 3: return <span className="text-orange-600 dark:text-orange-400 font-semibold">HIGH</span>;
      case 2: return <span className="text-yellow-600 dark:text-yellow-400 font-semibold">MEDIUM</span>;
      default: return <span className="text-slate-500">LOW</span>;
    }
  };

  const actionLabel = (action: number) => {
    switch (action) {
      case 0: return <span className="px-2 py-0.5 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 text-xs rounded-lg">BLOCK</span>;
      case 1: return <span className="px-2 py-0.5 bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 text-xs rounded-lg">WARN</span>;
      case 2: return <span className="px-2 py-0.5 bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400 text-xs rounded-lg">BYPASS</span>;
      default: return <span className="text-xs">-</span>;
    }
  };

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <PageHeader title="OPSEC Guard" subtitle="Pre-flight safety checks for agent operations">
        <div className="flex items-center gap-2">
          <span className="w-2 h-2 bg-emerald-500 rounded-full animate-pulse"></span>
          <span className="text-xs text-emerald-600 dark:text-emerald-400 font-medium">Active</span>
        </div>
      </PageHeader>

      <div className="ui-card p-6 mb-6">
        <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-4">Active Rules</h2>
        {loading ? (
          <div className="space-y-3">
            {[1,2,3,4].map(i => <div key={i} className="h-16 bg-slate-100 dark:bg-slate-700 rounded-xl animate-pulse" />)}
          </div>
        ) : rules.length === 0 ? (
          <p className="text-sm text-slate-500 dark:text-slate-400">No OPSEC rules configured</p>
        ) : (
          <div className="space-y-3">
            {rules.map((rule) => (
              <div key={rule.name} className="flex items-start gap-4 p-4 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-100 dark:border-slate-700">
                <div className="w-10 h-10 rounded-xl bg-indigo-100 dark:bg-indigo-900/30 flex items-center justify-center shrink-0">
                  <i className="fa-solid fa-shield-halved text-indigo-600 dark:text-indigo-400"></i>
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <code className="text-sm font-semibold text-slate-900 dark:text-slate-100">{rule.name}</code>
                    {riskLabel(rule.risk_level)}
                    {actionLabel(rule.default_action)}
                  </div>
                  <p className="text-xs text-slate-500 dark:text-slate-400">{rule.description}</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="ui-card p-6">
        <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-4">Quick Test</h2>
        <p className="text-xs text-slate-500 dark:text-slate-400 mb-4">Simulate an OPSEC check to verify rule behavior</p>
        <div className="flex flex-wrap gap-2">
          {["mimikatz", "creds", "inject", "shell", "ldap_users"].map((type) => (
            <button
              key={type}
              onClick={async () => {
                try {
                  const res = await fetch(`${API_BASE}?p=/api/opsec/check&format=json`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                      agent_id: "test-agent",
                      task_type: type,
                      username: "Administrator",
                      hostname: "DC-01",
                      ip: "10.0.1.5",
                      domain: "corp.local",
                      is_da: true,
                      processes: ["explorer.exe", "svchost.exe", "csfalcon.exe"],
                    }),
                  });
                  const data = await res.json();
                  setTestResult(data);
                } catch { setTestResult({ allowed: false, messages: "Test failed" }); }
              }}
              className={`px-4 h-10 rounded-xl text-xs font-medium transition-colors ${
                type === "mimikatz"
                  ? "bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 hover:bg-red-200 dark:hover:bg-red-900/50"
                  : "bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-600"
              }`}
            >
              <i className={`fa-solid mr-1.5 ${type === "mimikatz" ? "fa-skull" : "fa-bolt"}`}></i>
              {type}
            </button>
          ))}
        </div>
        {testResult && (
          <div className={`mt-4 p-4 rounded-xl text-sm ${
            testResult.allowed
              ? "bg-emerald-50 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800"
              : "bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400 border border-red-200 dark:border-red-800"
          }`}>
            <div className="flex items-center gap-2 mb-1">
              <i className={`fa-solid ${testResult.allowed ? "fa-check-circle" : "fa-circle-exclamation"}`}></i>
              <span className="font-semibold">{testResult.allowed ? "ALLOWED" : "BLOCKED"}</span>
            </div>
            <p className="text-xs opacity-80">{testResult.messages || "No issues"}</p>
          </div>
        )}
      </div>
    </div>
  );
}
