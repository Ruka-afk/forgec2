"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { API_BASE } from "@/lib/constants";
import { exportElementPng } from "@/lib/chartExport";

interface TopoNode {
  id?: string;
  group?: string;
  label?: string;
  title?: string;
}

interface TopoData {
  nodes?: TopoNode[];
  Nodes?: TopoNode[];
  edges?: Array<{ from: string; to: string }>;
  Edges?: Array<{ from: string; to: string }>;
}

declare global {
  interface Window {
    vis?: {
      Network: new (
        container: HTMLElement,
        data: unknown,
        options: unknown
      ) => {
        destroy: () => void;
        on: (event: string, callback: (params: { nodes: string[] }) => void) => void;
        setOptions: (options: unknown) => void;
        stabilize: () => void;
      };
    };
  }
}

export default function TopologyPage() {
  const [data, setData] = useState<TopoData | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedNode, setSelectedNode] = useState<TopoNode | null>(null);
  const [physicsEnabled, setPhysicsEnabled] = useState(true);
  const networkRef = useRef<HTMLDivElement>(null);
  const netInstanceRef = useRef<InstanceType<NonNullable<typeof window.vis>["Network"]> | null>(null);

  const loadTopology = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}?p=/api/topology/data&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const result = await res.json();
      if (result.data) {
        setData({ nodes: result.data.Nodes || result.data.nodes || [], edges: result.data.Edges || result.data.edges || [] });
      } else {
        setData({ nodes: result.Nodes || result.nodes || [], edges: result.Edges || result.edges || [] });
      }
    } catch {
      setData({ nodes: [], edges: [] });
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    Promise.resolve().then(() => {
      loadTopology();
      const interval = setInterval(loadTopology, 10000);
      return () => clearInterval(interval);
    });
  }, [loadTopology]);

  useEffect(() => {
    if (!networkRef.current || !data || loading) return;
    const nodes = data.nodes || data.Nodes || [];
    const edges = data.edges || data.Edges || [];
    if (nodes.length === 0) return;

    const init = () => {
      if (!window.vis?.Network || !networkRef.current) return;
      const visNodes = nodes.map((n, i) => ({
        id: n.id || String(i),
        label: n.label || n.id || "?",
        group: n.group || "default",
        title: n.title,
      }));
      const visEdges = edges.map((e, i) => ({ id: i, from: e.from, to: e.to }));
      const net = new window.vis.Network(
        networkRef.current,
        { nodes: visNodes, edges: visEdges },
        {
          physics: { enabled: physicsEnabled, stabilization: { iterations: 80 } },
          interaction: { hover: true },
          nodes: { font: { color: "#e2e8f0", size: 12 }, borderWidth: 2 },
          edges: { color: { color: "#6366f1" }, arrows: { to: { enabled: true, scaleFactor: 0.5 } } },
        }
      );
      netInstanceRef.current = net;
      net.on("click", (params: { nodes: string[] }) => {
        const nid = params.nodes[0];
        const found = nodes.find((n, i) => (n.id || String(i)) === nid);
        if (found) setSelectedNode(found);
      });
      return () => { net.destroy(); netInstanceRef.current = null; };
    };

    if (window.vis?.Network) {
      return init();
    }
    const script = document.createElement("script");
    script.src = "/js/vis-network.min.js";
    script.onload = () => init();
    document.head.appendChild(script);
    return () => { script.remove(); };
  }, [data, loading, physicsEnabled]);

  const togglePhysics = () => {
    const next = !physicsEnabled;
    setPhysicsEnabled(next);
    netInstanceRef.current?.setOptions({ physics: { enabled: next } });
  };

  const stabilizeGraph = () => {
    netInstanceRef.current?.stabilize();
  };

  const exportPng = async () => {
    if (!networkRef.current) return;
    try {
      await exportElementPng(networkRef.current, `topology-${Date.now()}.png`);
    } catch { /* ignore */ }
  };

  const nodes = data?.nodes || data?.Nodes || [];
  const edges = data?.edges || data?.Edges || [];
  const onlineCount = nodes.filter((n) => n.group === "agent-online").length;
  const offlineCount = nodes.filter((n) => n.group === "agent-offline").length;
  const listenerCount = nodes.filter((n) => n.group === "listener").length;

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-4 gap-3">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 bg-gradient-to-br from-indigo-500 to-purple-600 rounded-xl flex items-center justify-center shadow-lg shadow-indigo-500/20">
            <i className="fa-solid fa-diagram-project text-white text-sm"></i>
          </div>
          <div>
            <h1 className="text-xl font-bold text-slate-900 dark:text-slate-100">Network Topology</h1>
            <p className="text-slate-500 dark:text-slate-400 text-xs">Cobalt Strike Style Network View</p>
          </div>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-100 dark:bg-slate-800 rounded-lg">
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
            <span className="text-xs font-medium text-slate-600 dark:text-slate-300">{onlineCount} Online</span>
          </div>
          <div className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-100 dark:bg-slate-800 rounded-lg">
            <span className="w-2 h-2 rounded-full bg-slate-400"></span>
            <span className="text-xs font-medium text-slate-600 dark:text-slate-300">{offlineCount} Offline</span>
          </div>
          <div className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-100 dark:bg-slate-800 rounded-lg">
            <span className="w-2 h-2 rounded-full bg-indigo-500"></span>
            <span className="text-xs font-medium text-slate-600 dark:text-slate-300">{listenerCount} Listeners</span>
          </div>
          <button onClick={loadTopology} className="px-3 py-1.5 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 rounded-lg text-xs font-medium text-slate-600 dark:text-slate-300 transition-colors flex items-center gap-1.5">
            <i className="fa-solid fa-rotate"></i> Refresh
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-4 gap-4">
        <div className="xl:col-span-3">
          <div className="ui-card overflow-hidden shadow-sm">
            <div className="bg-gradient-to-r from-slate-800 via-slate-900 to-slate-800 px-4 py-2.5 flex items-center justify-between border-b border-slate-700">
              <div className="flex items-center gap-2">
                <div className="flex gap-1">
                  <span className="w-2.5 h-2.5 rounded-full bg-red-500"></span>
                  <span className="w-2.5 h-2.5 rounded-full bg-yellow-500"></span>
                  <span className="w-2.5 h-2.5 rounded-full bg-green-500"></span>
                </div>
                <span className="text-xs text-slate-400 font-mono ml-2">network-topology://view</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-[10px] text-slate-500 font-mono">ZOOM: 100%</span>
                <span className="text-[10px] text-slate-500 font-mono">NODES: {nodes.length}</span>
              </div>
            </div>
            <div ref={networkRef} className="relative p-4" style={{ minHeight: "500px", background: "radial-gradient(ellipse at center, #1e293b 0%, #0f172a 100%)" }}>
              {loading ? (
                <div className="absolute inset-0 flex items-center justify-center">
                  <div className="text-center">
                    <div className="animate-spin w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full mx-auto mb-3"></div>
                    <p className="text-slate-400 text-sm">Loading topology data...</p>
                  </div>
                </div>
              ) : nodes.length === 0 ? (
                <div className="absolute inset-0 flex items-center justify-center">
                  <div className="text-center">
                    <i className="fa-solid fa-network-wired text-4xl text-slate-600 mb-3"></i>
                    <p className="text-slate-400 text-sm">No topology data available</p>
                    <p className="text-slate-500 text-xs mt-1">Agents will appear here when they connect</p>
                  </div>
                </div>
              ) : null}
            </div>
            <div className="bg-slate-900 border-t border-slate-700 px-4 py-1.5 flex items-center justify-between">
              <span className="text-[10px] text-slate-500 font-mono">{loading ? "Loading..." : "Ready"}</span>
              <span className="text-[10px] text-slate-500 font-mono">{new Date().toLocaleTimeString()}</span>
            </div>
          </div>
        </div>

        <div className="space-y-4">
          <div className="ui-card overflow-hidden shadow-sm">
            <div className="bg-gradient-to-r from-indigo-600 to-purple-700 px-4 py-2.5">
              <div className="flex items-center gap-2">
                <i className="fa-solid fa-circle-info text-white text-xs"></i>
                <span className="text-xs font-semibold text-white">Node Details</span>
              </div>
            </div>
            <div className="p-4">
              {selectedNode ? (
                <div className="space-y-2">
                  <div className="font-semibold text-sm text-slate-900 dark:text-slate-100">{selectedNode.label}</div>
                  <div className="text-xs text-slate-500">{selectedNode.title?.replace(/<br\s*\/?>/g, "\n")}</div>
                </div>
              ) : (
                <div className="text-center py-6 text-slate-400 dark:text-slate-500">
                  <i className="fa-solid fa-arrow-pointer text-2xl mb-2 text-slate-300 dark:text-slate-600"></i>
                  <p className="text-xs">Click node to view details</p>
                </div>
              )}
            </div>
          </div>

          <div className="ui-card overflow-hidden shadow-sm">
            <div className="bg-gradient-to-r from-slate-700 to-slate-800 px-4 py-2.5">
              <div className="flex items-center gap-2">
                <i className="fa-solid fa-list text-white text-xs"></i>
                <span className="text-xs font-semibold text-white">Legend</span>
              </div>
            </div>
            <div className="p-4 space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="w-3 h-3 rounded-full bg-emerald-500 shadow-lg shadow-emerald-500/50"></span>
                  <span className="text-xs text-slate-600 dark:text-slate-300">Online Agent</span>
                </div>
                <span className="text-xs font-mono text-slate-400">{onlineCount}</span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="w-3 h-3 rounded-full bg-slate-500"></span>
                  <span className="text-xs text-slate-600 dark:text-slate-300">Offline Agent</span>
                </div>
                <span className="text-xs font-mono text-slate-400">{offlineCount}</span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="w-4 h-4 rounded bg-indigo-500 shadow-lg shadow-indigo-500/50" style={{ transform: "rotate(45deg)" }}></span>
                  <span className="text-xs text-slate-600 dark:text-slate-300">Listener</span>
                </div>
                <span className="text-xs font-mono text-slate-400">{listenerCount}</span>
              </div>
              <div className="border-t border-[var(--border)] pt-3">
                <div className="flex items-center gap-2 mb-2">
                  <span className="w-6 h-0.5 bg-slate-400 dark:bg-slate-500"></span>
                  <span className="text-xs text-slate-600 dark:text-slate-300">C2 Connection</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-6 h-0.5 border-t-2 border-dashed border-amber-500"></span>
                  <span className="text-xs text-slate-600 dark:text-slate-300">P2P Link</span>
                </div>
              </div>
            </div>
          </div>

          <div className="ui-card overflow-hidden shadow-sm">
            <div className="bg-gradient-to-r from-emerald-600 to-teal-700 px-4 py-2.5">
              <div className="flex items-center gap-2">
                <i className="fa-solid fa-bolt text-white text-xs"></i>
                <span className="text-xs font-semibold text-white">Quick Actions</span>
              </div>
            </div>
            <div className="p-4 space-y-2">
              <button onClick={togglePhysics} className="w-full px-3 py-2 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-lg text-xs font-medium text-slate-600 dark:text-slate-300 transition-colors flex items-center justify-between">
                <span><i className="fa-solid fa-snowflake mr-1.5 text-blue-400"></i>Physics Sim</span>
                <span className={`font-mono ${physicsEnabled ? "text-emerald-500" : "text-slate-400"}`}>{physicsEnabled ? "ON" : "OFF"}</span>
              </button>
              <button onClick={stabilizeGraph} className="w-full px-3 py-2 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-lg text-xs font-medium text-slate-600 dark:text-slate-300 transition-colors flex items-center justify-between">
                <span><i className="fa-solid fa-crosshairs mr-1.5 text-red-400"></i>Stabilize</span>
              </button>
              <button onClick={exportPng} className="w-full px-3 py-2 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-lg text-xs font-medium text-slate-600 dark:text-slate-300 transition-colors flex items-center justify-between">
                <span><i className="fa-solid fa-download mr-1.5 text-purple-400"></i>Export PNG</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
