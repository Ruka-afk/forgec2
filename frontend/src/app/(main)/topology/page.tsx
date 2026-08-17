"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import dynamic from "next/dynamic";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { exportElementPng } from "@/lib/chartExport";
import { nowTime } from "@/lib/utils";
import { PageContainer } from "@/components/ui/page-container";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Download, Info, List, Menu, MousePointerClick, RotateCw, Snowflake, Zap } from "lucide-react";
import type { TopoNode, TopoEdge, TopoData } from "@/types/topology";

const TopologyGraph = dynamic(() => import("@/components/TopologyGraph"), { ssr: false });

interface TopologySidebarProps {
  selectedTopoNode: TopoNode | null;
  selectedNode: TopoNode | null;
  setParentId: string;
  onSetParentId: (v: string) => void;
  handleSetRoute: () => void;
  routeMsg: string;
  onlineCount: number;
  offlineCount: number;
  listenerCount: number;
  meshNodeCount: number;
  useMeshSource: boolean;
  physicsEnabled: boolean;
  togglePhysics: () => void;
  exportPng: () => void;
}

function TopologySidebar({
  selectedTopoNode,
  selectedNode,
  setParentId,
  onSetParentId,
  handleSetRoute,
  routeMsg,
  onlineCount,
  offlineCount,
  listenerCount,
  meshNodeCount,
  useMeshSource,
  physicsEnabled,
  togglePhysics,
  exportPng,
}: TopologySidebarProps) {
  const { t } = useI18n();
  return (
    <>
      <Card className="overflow-hidden p-0">
        <div className="bg-primary/10 border border-primary/20 px-4 py-2.5">
          <div className="flex items-center gap-2">
            <Info className="w-4 h-4" />
            <span className="text-xs font-semibold text-foreground">{t("topology.node_details")}</span>
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
                  <div><b>{t("topology.p2p_mode")}</b> {selectedTopoNode.p2p_mode}</div>
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
                    aria-label={t("topology.parent_agent_id")} name="input-1"
                    value={setParentId}
                    onChange={(e) => onSetParentId(e.target.value)}
                    placeholder={t("topology.parent_agent_id")}
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
                  <p className={`text-xs mt-1 ${routeMsg.includes("success") ? "text-success" : "text-destructive"}`}>
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
        <div className="bg-secondary/60 border border-border px-4 py-2.5">
          <div className="flex items-center gap-2">
            <List className="w-4 h-4" />
            <span className="text-xs font-semibold text-foreground">{t("topology.legend")}</span>
          </div>
        </div>
        <div className="p-4 space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 rounded-full bg-success shadow-lg shadow-success/50"></span>
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
              <span className="w-4 h-4 rounded bg-primary/100 shadow-lg shadow-primary/50 rotate-45"></span>
              <span className="text-xs text-muted-foreground">{t("topology.legend_listener")}</span>
            </div>
            <span className="text-xs font-mono text-muted-foreground">{listenerCount}</span>
          </div>
          {useMeshSource && (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-chart-6"></span>
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
              <span className="w-6 h-0.5 border-t-2 border-dashed border-warning/50"></span>
              <span className="text-xs text-muted-foreground">{t("topology.edge_p2p")}</span>
            </div>
            <div className="flex items-center gap-2 mt-2">
              <span className="w-6 h-0.5 border-t-2 border-dashed border-success"></span>
              <span className="text-xs text-muted-foreground">{t("topology.edge_mesh")}</span>
            </div>
          </div>
        </div>
      </Card>

      <Card className="overflow-hidden p-0">
        <div className="bg-success/10 border border-success/20 px-4 py-2.5">
          <div className="flex items-center gap-2">
            <Zap className="w-4 h-4" />
            <span className="text-xs font-semibold text-foreground">{t("topology.quick_actions")}</span>
          </div>
        </div>
        <div className="p-4 space-y-2">
          <Button variant="ghost" size="sm" onClick={togglePhysics} className="w-full justify-between">
            <span><Snowflake className="w-4 h-4" />{t("topology.physics")}</span>
            <span className={`font-mono ${physicsEnabled ? "text-success" : "text-muted-foreground"}`}>{physicsEnabled ? t("topology.on") : t("topology.off")}</span>
          </Button>
          <Button variant="ghost" size="sm" onClick={exportPng} className="w-full justify-between">
            <span><Download className="w-4 h-4" />{t("topology.export_png")}</span>
          </Button>
        </div>
      </Card>
    </>
  );
}

export default function TopologyPage() {
  const { t } = useI18n();
  const [data, setData] = useState<TopoData | null>(null);
  const [meshData, setMeshData] = useState<TopoData | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedNode, setSelectedNode] = useState<TopoNode | null>(null);
  const [selectedTopoNode, setSelectedTopoNode] = useState<TopoNode | null>(null);
  const physicsRef = useRef(true);
  const [physicsEnabled, setPhysicsEnabled] = useState(true);
  const [useMeshSource, setUseMeshSource] = useState(true);
  const [setParentId, setSetParentId] = useState("");
  const [routeMsg, setRouteMsg] = useState("");
  const [now, setNow] = useState("");
  const [mobileSidebar, setMobileSidebar] = useState(false);

  const loadedOnceRef = useRef(false);

  const loadTopology = useCallback(async (signal?: AbortSignal) => {
    if (!loadedOnceRef.current) setLoading(true);
    try {
      if (useMeshSource) {
        const result = await api.get<{ success?: boolean; nodes?: TopoNode[]; edges?: TopoEdge[] }>("/mesh/topology", { signal });
        if (result.success) {
          setMeshData({ nodes: (result.nodes || []) as TopoNode[], edges: (result.edges || []) as TopoEdge[] });
        } else {
          setMeshData({ nodes: [] as TopoNode[], edges: [] as TopoEdge[] });
        }
      } else {
        const result = await api.get<{ Nodes?: TopoNode[]; nodes?: TopoNode[]; Edges?: Array<{ from: string; to: string }>; edges?: Array<{ from: string; to: string }>; data?: { Nodes?: TopoNode[]; nodes?: TopoNode[]; Edges?: Array<{ from: string; to: string }>; edges?: Array<{ from: string; to: string }> } }>(paths.topology.data, { signal });
        if (result.data) {
          setData({ nodes: (result.data.nodes || []) as TopoNode[], edges: (result.data.edges || []) as Array<{ from: string; to: string }> });
        } else {
          setData({ nodes: (result.nodes || []) as TopoNode[], edges: (result.edges || []) as Array<{ from: string; to: string }> });
        }
      }
    } catch {
      // Keep the last good graph on transient poll failures — wiping the
      // data here would destroy/recreate the vis instance (zoom/pan reset).
      // Only a first-load failure surfaces the empty state (data stays null).
    }
    loadedOnceRef.current = true;
    setLoading(false);
  }, [useMeshSource]);

  useEffect(() => {
    const controller = new AbortController();
    loadTopology(controller.signal);
    return () => controller.abort();
  }, [loadTopology]);
  useVisibleInterval(loadTopology, 10000);

  useEffect(() => { setNow(nowTime()); }, []);

  const graphContainerRef = useRef<HTMLDivElement>(null);

  const togglePhysics = () => {
    physicsRef.current = !physicsRef.current;
    setPhysicsEnabled(physicsRef.current);
  };

  const exportPng = async () => {
    if (!graphContainerRef.current) return;
    try {
      const el = graphContainerRef.current.querySelector("div") || graphContainerRef.current;
      await exportElementPng(el, `topology-${Date.now()}.png`);
    } catch { /* ignore */ }
  };

  const handleNodeClick = useCallback((node: TopoNode | null) => {
    if (!node) {
      setSelectedNode(null);
      setSelectedTopoNode(null);
      return;
    }
    if (useMeshSource) {
      setSelectedTopoNode(node);
      setSetParentId("");
      setRouteMsg("");
    } else {
      setSelectedNode(node);
    }
  }, [useMeshSource]);

  const handleSetRoute = async () => {
    if (!selectedTopoNode?.id || !setParentId) return;
    setRouteMsg("");
    try {
      const agentId = selectedTopoNode.id.replace("agent-", "");
      const result = await api.postJson(paths.mesh.route(agentId), { parent_id: setParentId });
      if (result.success) {
        setRouteMsg(t("topology.route_success"));
        loadTopology();
      } else {
        setRouteMsg(((result.error as string) || t("topology.route_failed")) as string);
      }
    } catch {
      setRouteMsg(t("topology.route_request_failed"));
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
    <PageContainer title={t("topology.title")} subtitle={useMeshSource ? t("topology.p2p_view") : t("topology.c2_view")} actions={<>
        <div className="flex items-center gap-1.5 px-3 py-1.5 bg-muted rounded-lg">
          <span className="w-2 h-2 rounded-full bg-success animate-pulse"></span>
          <span className="text-xs font-medium text-muted-foreground">{onlineCount} {t("topology.online")}</span>
        </div>
        <div className="flex items-center gap-1.5 px-3 py-1.5 bg-muted rounded-lg">
          <span className="w-2 h-2 rounded-full bg-muted-foreground"></span>
          <span className="text-xs font-medium text-muted-foreground">{offlineCount} {t("topology.offline")}</span>
        </div>
        <div className="flex items-center gap-1.5 px-3 py-1.5 bg-muted rounded-lg">
          <span className="w-2 h-2 rounded-full bg-primary/100"></span>
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
          <Button variant="ghost" size="icon" aria-label={t("topology.open_sidebar")} className="min-w-[44px] min-h-[44px]" onClick={() => setMobileSidebar(true)}>
            <Menu className="w-5 h-5" />
          </Button>
        </div>
        <Button variant="ghost" size="xs" onClick={() => loadTopology()} className="flex items-center gap-1.5">
          <RotateCw className="w-4 h-4" /> {t("topology.refresh")}
        </Button>
      </>}>

      <div className="grid grid-cols-1 xl:grid-cols-4 gap-4">
        <div className="xl:col-span-3">
          <Card className="overflow-hidden p-0">
            <div className="bg-card border-b border-border px-4 py-2.5 flex items-center justify-between border-b border-border">
              <div className="flex items-center gap-2">
                <div className="flex gap-1">
                  <span className="w-2.5 h-2.5 rounded-full bg-destructive"></span>
                  <span className="w-2.5 h-2.5 rounded-full bg-chart-4"></span>
                  <span className="w-2.5 h-2.5 rounded-full bg-success"></span>
                </div>
                <span className="text-xs text-muted-foreground font-mono ml-2">network-topology://view</span>
              </div>
              <div className="flex items-center gap-2">
                <Label className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Checkbox aria-label={t("topology.use_mesh")} checked={useMeshSource} onCheckedChange={() => setUseMeshSource(!useMeshSource)} />
                  Mesh DB
                </Label>
                <span className="text-(--fs-micro-sm) text-muted-foreground font-mono">NODES: {nodes.length}</span>
              </div>
            </div>
            <div ref={graphContainerRef} className="relative p-4 bg-card [background-image:radial-gradient(ellipse_at_center,var(--card)_0%,var(--background)_100%)] min-h-[500px]">
              <TopologyGraph
                data={data || { nodes: [], edges: [] }}
                meshData={meshData || { nodes: [], edges: [] }}
                useMeshSource={useMeshSource}
                physicsEnabled={physicsEnabled}
                loading={loading}
                onNodeClick={handleNodeClick}
              />
            </div>
            <div className="bg-background border-t border-border px-4 py-1.5 flex items-center justify-between">
              <span className="text-(--fs-micro-sm) text-muted-foreground font-mono">
                {loading ? t("common.loading") : useMeshSource ? "Mesh topology from DB" : "C2 topology from agent data"}
              </span>
              <span className="text-(--fs-micro-sm) text-muted-foreground font-mono">{now || "--:--:--"}</span>
            </div>
          </Card>
        </div>

        <div className="hidden xl:block space-y-4">
          <TopologySidebar
            selectedTopoNode={selectedTopoNode}
            selectedNode={selectedNode}
            setParentId={setParentId}
            onSetParentId={setSetParentId}
            handleSetRoute={handleSetRoute}
            routeMsg={routeMsg}
            onlineCount={onlineCount}
            offlineCount={offlineCount}
            listenerCount={listenerCount}
            meshNodeCount={meshNodeCount}
            useMeshSource={useMeshSource}
            physicsEnabled={physicsEnabled}
            togglePhysics={togglePhysics}
            exportPng={exportPng}
          />
        </div>
      </div>

      <Sheet open={mobileSidebar} onOpenChange={setMobileSidebar}>
        <SheetContent side="right" className="w-[320px] sm:w-[360px] overflow-y-auto">
          <div className="space-y-4 pt-8">
            <TopologySidebar
              selectedTopoNode={selectedTopoNode}
              selectedNode={selectedNode}
              setParentId={setParentId}
              onSetParentId={setSetParentId}
              handleSetRoute={handleSetRoute}
              routeMsg={routeMsg}
              onlineCount={onlineCount}
              offlineCount={offlineCount}
              listenerCount={listenerCount}
              meshNodeCount={meshNodeCount}
              useMeshSource={useMeshSource}
              physicsEnabled={physicsEnabled}
              togglePhysics={togglePhysics}
              exportPng={exportPng}
            />
          </div>
        </SheetContent>
      </Sheet>
    </PageContainer>
  );
}
