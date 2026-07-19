"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { exportElementPng } from "@/lib/chartExport";
import { PageHeader, Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { Crosshair, Download, Info, List, Menu, MousePointerClick, Network, RotateCw, Snowflake, Zap } from "lucide-react";

interface TopoNode {
  id?: string;
  label?: string;
  group?: string;
  title?: string;
  p2p_mode?: string;
  peer_count?: number;
}

interface TopoEdge {
  from: string;
  to: string;
  dashes?: boolean;
  color?: string;
  title?: string;
  width?: number;
  length?: number;
}

interface TopoData {
  nodes?: TopoNode[];
  Nodes?: TopoNode[];
  edges?: TopoEdge[];
  Edges?: TopoEdge[];
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
        setData: (data: { nodes: unknown; edges: unknown }) => void;
        stabilize: () => void;
      };
    };
  }
}

export default function TopologyPage() {
  const { t } = useI18n();
  const [data, setData] = useState<TopoData | null>(null);
  const [meshData, setMeshData] = useState<TopoData | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedNode, setSelectedNode] = useState<TopoNode | null>(null);
  const [selectedTopoNode, setSelectedTopoNode] = useState<TopoNode | null>(null);
  const [physicsEnabled, setPhysicsEnabled] = useState(true);
  const [useMeshSource, setUseMeshSource] = useState(true);
  const [setParentId, setSetParentId] = useState("");
  const [routeMsg, setRouteMsg] = useState("");
  const [now, setNow] = useState("");
  const [mobileSidebar, setMobileSidebar] = useState(false);
  const networkRef = useRef<HTMLDivElement>(null);
  const netInstanceRef = useRef<InstanceType<NonNullable<typeof window.vis>["Network"]> | null>(null);

  const loadTopology = useCallback(async () => {
    setLoading(true);
    try {
      if (useMeshSource) {
        const result = await api.json<{ success?: boolean; nodes?: TopoNode[]; edges?: TopoEdge[] }>("/mesh/topology");
        if (result.success) {
          setMeshData({ nodes: (result.nodes || []) as TopoNode[], edges: (result.edges || []) as TopoEdge[] });
        } else {
          setMeshData({ nodes: [] as TopoNode[], edges: [] as TopoEdge[] });
        }
      } else {
        const result = await api.get<{ Nodes?: TopoNode[]; nodes?: TopoNode[]; Edges?: Array<{ from: string; to: string }>; edges?: Array<{ from: string; to: string }>; data?: { Nodes?: TopoNode[]; nodes?: TopoNode[]; Edges?: Array<{ from: string; to: string }>; edges?: Array<{ from: string; to: string }> } }>("/api/topology/data");
        if (result.data) {
          setData({ nodes: (result.data.nodes || []) as TopoNode[], edges: (result.data.edges || []) as Array<{ from: string; to: string }> });
        } else {
          setData({ nodes: (result.nodes || []) as TopoNode[], edges: (result.edges || []) as Array<{ from: string; to: string }> });
        }
      }
    } catch {
      setData({ nodes: [], edges: [] });
      setMeshData({ nodes: [], edges: [] });
    }
    setLoading(false);
  }, [useMeshSource]);

  useEffect(() => { loadTopology(); }, [loadTopology]);
  useVisibleInterval(loadTopology, 10000);

  useEffect(() => { setNow(new Date().toLocaleTimeString()); }, []);

  useEffect(() => {
    if (!networkRef.current || loading) return;

    const nodes = useMeshSource
      ? (meshData?.nodes || [])
      : (data?.nodes || data?.Nodes || []);
    const edges = useMeshSource
      ? (meshData?.edges || [])
      : (data?.edges || data?.Edges || []);
    if (nodes.length === 0) return;

    const buildVisData = () => {
      const textColor = getComputedStyle(document.documentElement).getPropertyValue('--foreground').trim() || '#e2e8f0';
      const visNodes = useMeshSource
        ? (nodes as TopoNode[]).map((n, i) => ({
            id: n.id || String(i),
            label: n.label,
            group: n.group || "default",
            title: n.title || n.label,
          }))
        : (nodes as TopoNode[]).map((n, i) => ({
            id: n.id || String(i),
            label: n.label || n.id || "?",
            group: n.group || "default",
            title: n.title,
          }));
      const visEdges = edges.map((e, i) => ({ id: i, from: e.from, to: e.to }));
      return { visNodes, visEdges, textColor };
    };

    const init = () => {
      if (!window.vis?.Network || !networkRef.current) return;
      if (netInstanceRef.current) {
        const { visNodes, visEdges } = buildVisData();
        netInstanceRef.current.setData({ nodes: visNodes, edges: visEdges });
        return;
      }
      const { visNodes, visEdges, textColor } = buildVisData();
      const net = new window.vis.Network(
        networkRef.current,
        { nodes: visNodes, edges: visEdges },
        {
          physics: { enabled: physicsEnabled, stabilization: { iterations: 80 } },
          interaction: { hover: true },
          nodes: { font: { color: textColor, size: 12 }, borderWidth: 2 },
          edges: { color: { color: "#6366f1" }, arrows: { to: { enabled: true, scaleFactor: 0.5 } } },
        }
      );
      netInstanceRef.current = net;
      net.on("click", (params: { nodes: string[] }) => {
        const nid = params.nodes[0];
        if (useMeshSource) {
          const found = (nodes as TopoNode[]).find((n) => n.id === nid);
          if (found) {
            setSelectedTopoNode(found);
            setSetParentId("");
            setRouteMsg("");
          }
        } else {
          const found = (nodes as TopoNode[]).find((n, i) => (n.id || String(i)) === nid);
          if (found) setSelectedNode(found);
        }
      });
    };

    if (window.vis?.Network) {
      init();
      return () => { netInstanceRef.current?.destroy(); netInstanceRef.current = null; };
    }
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = "/js/vis-network.min.css";
    document.head.appendChild(link);
    const script = document.createElement("script");
    script.src = "/js/vis-network.min.js";
    let destroyed = false;
    script.onload = () => { if (!destroyed) init(); };
    document.head.appendChild(script);
    return () => {
      destroyed = true;
      netInstanceRef.current?.destroy();
      netInstanceRef.current = null;
      script.remove();
      link.remove();
    };
  }, [data, meshData, loading, physicsEnabled, useMeshSource]);

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

  const handleSetRoute = async () => {
    if (!selectedTopoNode?.id || !setParentId) return;
    setRouteMsg("");
    try {
      const agentId = selectedTopoNode.id.replace("agent-", "");
      const result = await api.postJson(`/mesh/route/${agentId}`, { parent_id: setParentId });
      if (result.success) {
        setRouteMsg("Route updated successfully!");
        loadTopology();
      } else {
        setRouteMsg(((result.error as string) || "Failed to set route") as string);
      }
    } catch {
      setRouteMsg("Request failed");
    }
  };

  const nodes = useMeshSource
    ? (meshData?.nodes || [])
    : (data?.nodes || data?.Nodes || []);
  const onlineCount = nodes.filter((n) => n.group === "agent-online").length;
  const offlineCount = nodes.filter((n) => n.group === "agent-offline").length;
  const listenerCount = nodes.filter((n) => n.group === "listener").length;
  const meshNodeCount = useMeshSource ? (meshData?.nodes || []).length : 0;
  const meshEdgeCount = useMeshSource ? (meshData?.edges || []).length : 0;

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("topology.title")} subtitle={useMeshSource ? t("topology.p2p_view") : t("topology.c2_view")}>
        <div className="flex items-center gap-1.5 px-3 py-1.5 bg-muted rounded-lg">
          <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
          <span className="text-xs font-medium text-muted-foreground">{onlineCount} {t("topology.online")}</span>
        </div>
        <div className="flex items-center gap-1.5 px-3 py-1.5 bg-muted rounded-lg">
          <span className="w-2 h-2 rounded-full bg-muted-foreground"></span>
          <span className="text-xs font-medium text-muted-foreground">{offlineCount} {t("topology.offline")}</span>
        </div>
        <div className="flex items-center gap-1.5 px-3 py-1.5 bg-muted rounded-lg">
          <span className="w-2 h-2 rounded-full bg-indigo-500"></span>
          <span className="text-xs font-medium text-muted-foreground">{listenerCount} {t("topology.listeners")}</span>
        </div>
        {useMeshSource && (
          <div className="flex items-center gap-1.5 px-3 py-1.5 bg-muted rounded-lg">
            <span className="text-xs font-medium text-muted-foreground">
              {meshNodeCount}N / {meshEdgeCount}E
            </span>
          </div>
        )}
        <div className="xl:hidden">
          <Button variant="ghost" size="icon" className="min-w-[44px] min-h-[44px]" onClick={() => setMobileSidebar(true)}>
            <Menu className="w-5 h-5" />
          </Button>
        </div>
        <Button variant="ghost" size="xs" onClick={loadTopology} className="flex items-center gap-1.5">
          <RotateCw className="w-4 h-4" /> {t("topology.refresh")}
        </Button>
      </PageHeader>

      <div className="grid grid-cols-1 xl:grid-cols-4 gap-4">
        <div className="xl:col-span-3">
          <Card className="overflow-hidden p-0">
            <div className="bg-gradient-to-r from-card via-background to-card px-4 py-2.5 flex items-center justify-between border-b border-border">
              <div className="flex items-center gap-2">
                <div className="flex gap-1">
                  <span className="w-2.5 h-2.5 rounded-full bg-red-500"></span>
                  <span className="w-2.5 h-2.5 rounded-full bg-yellow-500"></span>
                  <span className="w-2.5 h-2.5 rounded-full bg-emerald-500"></span>
                </div>
                <span className="text-xs text-muted-foreground font-mono ml-2">network-topology://view</span>
              </div>
              <div className="flex items-center gap-2">
                <Label className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Checkbox aria-label="Use mesh database source" checked={useMeshSource} onCheckedChange={() => setUseMeshSource(!useMeshSource)} />
                  Mesh DB
                </Label>
                <span className="text-[10px] text-muted-foreground font-mono">NODES: {nodes.length}</span>
              </div>
            </div>
            <div ref={networkRef} className="relative p-4 bg-card [background-image:radial-gradient(ellipse_at_center,var(--card)_0%,var(--background)_100%)] min-h-[500px]">
              {loading ? (
                <div className="absolute inset-0 flex items-center justify-center">
                  <div className="text-center">
                    <Spinner size="md" className="mx-auto mb-3" />
                    <p className="text-muted-foreground text-sm">{t("topology.loading")}</p>
                  </div>
                </div>
              ) : nodes.length === 0 ? (
                <div className="absolute inset-0 flex items-center justify-center">
                  <div className="text-center">
                    <Network className="w-4 h-4" />
                    <p className="text-muted-foreground text-sm">{t("topology.empty")}</p>
                    <p className="text-muted-foreground text-xs mt-1">{t("topology.empty_hint")}</p>
                  </div>
                </div>
              ) : null}
            </div>
            <div className="bg-background border-t border-border px-4 py-1.5 flex items-center justify-between">
              <span className="text-[10px] text-muted-foreground font-mono">
                {loading ? t("common.loading") : useMeshSource ? "Mesh topology from DB" : "C2 topology from agent data"}
              </span>
              <span className="text-[10px] text-muted-foreground font-mono">{now || "--:--:--"}</span>
            </div>
          </Card>
        </div>

        <div className="hidden xl:block space-y-4">
          <Card className="overflow-hidden p-0">
            <div className="bg-gradient-to-r from-indigo-600 to-purple-700 px-4 py-2.5">
              <div className="flex items-center gap-2">
                <Info className="w-4 h-4" />
                <span className="text-xs font-semibold text-white">{t("topology.node_details")}</span>
              </div>
            </div>
            <div className="p-4">
              {selectedTopoNode ? (
                <div className="space-y-2">
                  <div className="font-semibold text-sm text-foreground">
                    {selectedTopoNode.label || selectedTopoNode.id}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    <div><b>ID:</b> {selectedTopoNode.id}</div>
                    <div><b>{t("topology.group")}:</b> {selectedTopoNode.group}</div>
                    {selectedTopoNode.p2p_mode && (
                      <div><b>P2P Mode:</b> {selectedTopoNode.p2p_mode}</div>
                    )}
                    <div><b>{t("topology.peers")}:</b> {selectedTopoNode.peer_count ?? "?"}</div>
                    {selectedTopoNode.title && (
                      <div className="mt-1 whitespace-pre-line">{selectedTopoNode.title}</div>
                    )}
                  </div>
                  <div className="mt-3">
                    <span className="text-xs text-muted-foreground block mb-1">{t("topology.set_parent")}</span>
                    <div className="flex gap-1">
                      <Input
                        aria-label="parent agent ID" name="input-1"
                        value={setParentId}
                        onChange={(e) => setSetParentId(e.target.value)}
                        placeholder="parent agent ID"
                        className="flex-1 text-xs"
                      />
                      <Button
                        size="xs"
                        onClick={handleSetRoute}
                      >
                        {t("topology.set")}
                      </Button>
                    </div>
                    {routeMsg && (
                      <p className={`text-xs mt-1 ${routeMsg.includes("success") ? "text-emerald-500" : "text-red-500"}`}>
                        {routeMsg}
                      </p>
                    )}
                  </div>
                </div>
              ) : selectedNode ? (
                <div className="space-y-2">
                  <div className="font-semibold text-sm text-foreground">{selectedNode.label}</div>
                  <div className="text-xs text-muted-foreground">{selectedNode.title?.replace(/<br\s*\/?>/g, "\n")}</div>
                </div>
              ) : (
                <div className="text-center py-6 text-muted-foreground">
                  <MousePointerClick className="w-4 h-4" />
                  <p className="text-xs">{t("topology.click_hint")}</p>
                </div>
              )}
            </div>
          </Card>

          <Card className="overflow-hidden p-0">
            <div className="bg-gradient-to-r from-muted to-secondary px-4 py-2.5">
              <div className="flex items-center gap-2">
                <List className="w-4 h-4" />
                <span className="text-xs font-semibold text-white">{t("topology.legend")}</span>
              </div>
            </div>
            <div className="p-4 space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="w-3 h-3 rounded-full bg-emerald-500 shadow-lg shadow-emerald-500/50"></span>
                  <span className="text-xs text-muted-foreground">{t("topology.legend_online")}</span>
                </div>
                <span className="text-xs font-mono text-muted-foreground">{onlineCount}</span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="w-3 h-3 rounded-full bg-muted-foreground"></span>
                  <span className="text-xs text-muted-foreground">{t("topology.legend_offline")}</span>
                </div>
                <span className="text-xs font-mono text-muted-foreground">{offlineCount}</span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="w-4 h-4 rounded bg-indigo-500 shadow-lg shadow-indigo-500/50 rotate-45"></span>
                  <span className="text-xs text-muted-foreground">{t("topology.legend_listener")}</span>
                </div>
                <span className="text-xs font-mono text-muted-foreground">{listenerCount}</span>
              </div>
              {useMeshSource && (
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="w-3 h-3 rounded-full bg-violet-500"></span>
                    <span className="text-xs text-muted-foreground">{t("topology.legend_peer")}</span>
                  </div>
                  <span className="text-xs font-mono text-muted-foreground">{meshNodeCount}</span>
                </div>
              )}
              <div className="border-t border-border pt-3">
                <div className="flex items-center gap-2 mb-2">
                  <span className="w-6 h-0.5 bg-muted-foreground"></span>
                  <span className="text-xs text-muted-foreground">{t("topology.edge_c2")}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-6 h-0.5 border-t-2 border-dashed border-amber-500"></span>
                  <span className="text-xs text-muted-foreground">{t("topology.edge_p2p")}</span>
                </div>
                <div className="flex items-center gap-2 mt-2">
                  <span className="w-6 h-0.5 border-t-2 border-dashed border-emerald-500"></span>
                  <span className="text-xs text-muted-foreground">{t("topology.edge_mesh")}</span>
                </div>
              </div>
            </div>
          </Card>

          <Card className="overflow-hidden p-0">
            <div className="bg-gradient-to-r from-emerald-600 to-teal-700 px-4 py-2.5">
              <div className="flex items-center gap-2">
                <Zap className="w-4 h-4" />
                <span className="text-xs font-semibold text-white">{t("topology.quick_actions")}</span>
              </div>
            </div>
            <div className="p-4 space-y-2">
              <Button variant="ghost" size="sm" onClick={togglePhysics} className="w-full justify-between">
                <span><Snowflake className="w-4 h-4" />{t("topology.physics")}</span>
                <span className={`font-mono ${physicsEnabled ? "text-emerald-500" : "text-muted-foreground"}`}>{physicsEnabled ? t("topology.on") : t("topology.off")}</span>
              </Button>
              <Button variant="ghost" size="sm" onClick={stabilizeGraph} className="w-full justify-between">
                <span><Crosshair className="w-4 h-4" />{t("topology.stabilize")}</span>
              </Button>
              <Button variant="ghost" size="sm" onClick={exportPng} className="w-full justify-between">
                <span><Download className="w-4 h-4" />{t("topology.export_png")}</span>
              </Button>
            </div>
          </Card>
        </div>
      </div>

      <Sheet open={mobileSidebar} onOpenChange={setMobileSidebar}>
        <SheetContent side="right" className="w-[320px] sm:w-[360px] overflow-y-auto">
          <div className="space-y-4 pt-8">
            <Card className="overflow-hidden p-0">
              <div className="bg-gradient-to-r from-indigo-600 to-purple-700 px-4 py-2.5">
                <div className="flex items-center gap-2">
                  <Info className="w-4 h-4" />
                  <span className="text-xs font-semibold text-white">{t("topology.node_details")}</span>
                </div>
              </div>
              <div className="p-4">
                {selectedTopoNode ? (
                  <div className="space-y-2">
                    <div className="font-semibold text-sm text-foreground">
                      {selectedTopoNode.label || selectedTopoNode.id}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      <div><b>ID:</b> {selectedTopoNode.id}</div>
                      <div><b>{t("topology.group")}:</b> {selectedTopoNode.group}</div>
                      {selectedTopoNode.p2p_mode && (
                        <div><b>P2P Mode:</b> {selectedTopoNode.p2p_mode}</div>
                      )}
                      <div><b>{t("topology.peers")}:</b> {selectedTopoNode.peer_count ?? "?"}</div>
                      {selectedTopoNode.title && (
                        <div className="mt-1 whitespace-pre-line">{selectedTopoNode.title}</div>
                      )}
                    </div>
                    <div className="mt-3">
                      <span className="text-xs text-muted-foreground block mb-1">{t("topology.set_parent")}</span>
                      <div className="flex gap-1">
                        <Input
                          aria-label="parent agent ID" name="input-1"
                          value={setParentId}
                          onChange={(e) => setSetParentId(e.target.value)}
                          placeholder="parent agent ID"
                          className="flex-1 text-xs"
                        />
                        <Button size="xs" onClick={handleSetRoute}>{t("topology.set")}</Button>
                      </div>
                      {routeMsg && (
                        <p className={`text-xs mt-1 ${routeMsg.includes("success") ? "text-emerald-500" : "text-red-500"}`}>
                          {routeMsg}
                        </p>
                      )}
                    </div>
                  </div>
                ) : selectedNode ? (
                  <div className="space-y-2">
                    <div className="font-semibold text-sm text-foreground">{selectedNode.label}</div>
                    <div className="text-xs text-muted-foreground">{selectedNode.title?.replace(/<br\s*\/?>/g, "\n")}</div>
                  </div>
                ) : (
                  <div className="text-center py-6 text-muted-foreground">
                    <MousePointerClick className="w-4 h-4 mx-auto" />
                    <p className="text-xs">{t("topology.click_hint")}</p>
                  </div>
                )}
              </div>
            </Card>

            <Card className="overflow-hidden p-0">
              <div className="bg-gradient-to-r from-muted to-secondary px-4 py-2.5">
                <div className="flex items-center gap-2">
                  <List className="w-4 h-4" />
                  <span className="text-xs font-semibold text-white">{t("topology.legend")}</span>
                </div>
              </div>
              <div className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="w-3 h-3 rounded-full bg-emerald-500 shadow-lg shadow-emerald-500/50"></span>
                    <span className="text-xs text-muted-foreground">{t("topology.legend_online")}</span>
                  </div>
                  <span className="text-xs font-mono text-muted-foreground">{onlineCount}</span>
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="w-3 h-3 rounded-full bg-muted-foreground"></span>
                    <span className="text-xs text-muted-foreground">{t("topology.legend_offline")}</span>
                  </div>
                  <span className="text-xs font-mono text-muted-foreground">{offlineCount}</span>
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="w-4 h-4 rounded bg-indigo-500 shadow-lg shadow-indigo-500/50 rotate-45"></span>
                    <span className="text-xs text-muted-foreground">{t("topology.legend_listener")}</span>
                  </div>
                  <span className="text-xs font-mono text-muted-foreground">{listenerCount}</span>
                </div>
              </div>
            </Card>

            <Card className="overflow-hidden p-0">
              <div className="bg-gradient-to-r from-emerald-600 to-teal-700 px-4 py-2.5">
                <div className="flex items-center gap-2">
                  <Zap className="w-4 h-4" />
                  <span className="text-xs font-semibold text-white">{t("topology.quick_actions")}</span>
                </div>
              </div>
              <div className="p-4 space-y-2">
                <Button variant="ghost" size="sm" onClick={togglePhysics} className="w-full justify-between">
                  <span><Snowflake className="w-4 h-4" />{t("topology.physics")}</span>
                  <span className={`font-mono ${physicsEnabled ? "text-emerald-500" : "text-muted-foreground"}`}>{physicsEnabled ? t("topology.on") : t("topology.off")}</span>
                </Button>
                <Button variant="ghost" size="sm" onClick={stabilizeGraph} className="w-full justify-between">
                  <span><Crosshair className="w-4 h-4" />{t("topology.stabilize")}</span>
                </Button>
                <Button variant="ghost" size="sm" onClick={exportPng} className="w-full justify-between">
                  <span><Download className="w-4 h-4" />{t("topology.export_png")}</span>
                </Button>
              </div>
            </Card>
          </div>
        </SheetContent>
      </Sheet>
    </div>
  );
}
