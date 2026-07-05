"use client";

import { useEffect, useState } from "react";
import { API_BASE } from "@/lib/constants";
import { PageHeader } from "@/components/UI";

interface ListenerStatus {
  id: string;
  health: string;
}

export default function CircuitBreakerPage() {
  const [listeners, setListeners] = useState<ListenerStatus[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const load = () => {
      fetch(`${API_BASE}?p=/api/circuit-breaker/status&format=json`)
        .then((r) => r.json())
        .then((data) => setListeners(data.listeners || []))
        .catch(() => setListeners([]))
        .finally(() => setLoading(false));
    };
    load();
    const iv = setInterval(load, 15000);
    return () => clearInterval(iv);
  }, []);

  const healthColor = (h: string) => {
    switch (h) {
      case "healthy": return { dot: "bg-emerald-500", bg: "bg-emerald-50 dark:bg-emerald-900/20 border-emerald-200 dark:border-emerald-800", text: "text-emerald-700 dark:text-emerald-400", icon: "fa-shield-check" };
      case "unstable": return { dot: "bg-amber-500", bg: "bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800", text: "text-amber-700 dark:text-amber-400", icon: "fa-triangle-exclamation" };
      case "burned": return { dot: "bg-red-500", bg: "bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800", text: "text-red-700 dark:text-red-400", icon: "fa-circle-radiation" };
      default: return { dot: "bg-slate-400", bg: "bg-slate-50 dark:bg-slate-800 border-slate-200 dark:border-slate-700", text: "text-slate-500 dark:text-slate-400", icon: "fa-circle-question" };
    }
  };

  const healthyCount = listeners.filter((l) => l.health === "healthy").length;
  const burnedCount = listeners.filter((l) => l.health === "burned").length;

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <PageHeader title="Circuit Breaker" subtitle="Real-time listener health monitoring with auto-failover">
        <div className="flex items-center gap-3">
          <span className="flex items-center gap-1.5 text-xs">
            <span className="w-2 h-2 bg-emerald-500 rounded-full"></span>
            {healthyCount} Healthy
          </span>
          {burnedCount > 0 && (
            <span className="flex items-center gap-1.5 text-xs text-red-600 dark:text-red-400">
              <span className="w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
              {burnedCount} Burned
            </span>
          )}
        </div>
      </PageHeader>

      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1,2,3].map(i => <div key={i} className="h-32 bg-slate-100 dark:bg-slate-700 rounded-2xl animate-pulse" />)}
        </div>
      ) : listeners.length === 0 ? (
        <div className="ui-card p-12 text-center">
          <i className="fa-solid fa-shield text-4xl text-slate-300 dark:text-slate-600 mb-3"></i>
          <p className="text-slate-500 dark:text-slate-400 text-sm">No listeners registered for health monitoring</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {listeners.map((l) => {
            const c = healthColor(l.health);
            return (
              <div key={l.id} className={`ui-card p-5 border ${c.bg} transition-all`}>
                <div className="flex items-start justify-between mb-3">
                  <div className={`w-10 h-10 rounded-xl ${c.bg} flex items-center justify-center`}>
                    <i className={`fa-solid ${c.icon} ${c.text} text-lg`}></i>
                  </div>
                  <span className={`w-3 h-3 rounded-full ${c.dot} ${l.health === "burned" ? "animate-pulse" : ""}`}></span>
                </div>
                <div className="font-mono text-sm font-semibold text-slate-900 dark:text-slate-100 mb-1">Listener #{l.id}</div>
                <div className={`text-xs font-medium ${c.text}`}>
                  <i className="fa-solid fa-circle mr-1 text-[6px]"></i>
                  {l.health.toUpperCase()}
                </div>
              </div>
            );
          })}
        </div>
      )}

      <div className="mt-6 ui-card p-6">
        <h3 className="text-sm font-semibold text-slate-900 dark:text-slate-100 mb-3">How Circuit Breaker Works</h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs text-slate-500 dark:text-slate-400">
          <div className="p-4 bg-slate-50 dark:bg-slate-800/50 rounded-xl">
            <div className="text-emerald-500 font-semibold mb-1 text-sm">1. Probe</div>
            External vantage points probe each listener every 60s (TCP/TLS/HTTP)
          </div>
          <div className="p-4 bg-slate-50 dark:bg-slate-800/50 rounded-xl">
            <div className="text-amber-500 font-semibold mb-1 text-sm">2. Detect</div>
            3+ consecutive failures = UNSTABLE. TCP RST or content injection = BURNED
          </div>
          <div className="p-4 bg-slate-50 dark:bg-slate-800/50 rounded-xl">
            <div className="text-red-500 font-semibold mb-1 text-sm">3. Respond</div>
            BURNED triggers automatic profile rotation for all connected agents
          </div>
        </div>
      </div>
    </div>
  );
}
