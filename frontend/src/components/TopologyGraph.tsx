"use client";

import { useEffect, useRef, useMemo } from "react";
import { Spinner } from "@/components/ui/spinner";
import { Network } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { TopoNode, TopoData } from "@/types/topology";

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

interface TopologyGraphProps {
  data: TopoData;
  meshData: TopoData;
  useMeshSource: boolean;
  physicsEnabled: boolean;
  loading: boolean;
  onNodeClick: (node: TopoNode | null) => void;
}

export default function TopologyGraph({
  data,
  meshData,
  useMeshSource,
  physicsEnabled,
  loading,
  onNodeClick,
}: TopologyGraphProps) {
  const { t } = useI18n();
  const networkRef = useRef<HTMLDivElement>(null);
  const netInstanceRef = useRef<InstanceType<NonNullable<typeof window.vis>["Network"]> | null>(null);

  const nodes = useMemo(() => (useMeshSource
    ? (meshData?.nodes || [])
    : (data?.nodes || [])),
  [useMeshSource, meshData?.nodes, data?.nodes]);
  const edges = useMemo(() => (useMeshSource
    ? (meshData?.edges || [])
    : (data?.edges || [])),
  [useMeshSource, meshData?.edges, data?.edges]);

  // Refs mirror the latest props so the vis instance never needs re-creation
  // for data/callback/physics changes (re-creation resets zoom/pan/selection).
  const nodesRef = useRef(nodes);
  nodesRef.current = nodes;
  const edgesRef = useRef(edges);
  edgesRef.current = edges;
  const onNodeClickRef = useRef(onNodeClick);
  onNodeClickRef.current = onNodeClick;
  const physicsRef = useRef(physicsEnabled);
  physicsRef.current = physicsEnabled;

  const buildVisData = () => {
    const textColor =
      getComputedStyle(document.documentElement)
        .getPropertyValue("--foreground")
        .trim() || "#e2e8f0";
    const currentNodes = nodesRef.current as TopoNode[];
    const visNodes = currentNodes.map((n, i) => ({
      id: n.id || String(i),
      label: n.label || n.id || "?",
      group: n.group || "default",
      title: n.title,
    }));
    const visEdges = edgesRef.current.map((e, i) => ({ id: i, from: e.from, to: e.to }));
    return { visNodes, visEdges, textColor };
  };

  const syncData = () => {
    const net = netInstanceRef.current;
    if (!net) return;
    const { visNodes, visEdges } = buildVisData();
    net.setData({ nodes: visNodes, edges: visEdges });
    net.setOptions({ physics: { enabled: physicsRef.current } });
  };

  const showGraph = !loading && nodes.length > 0;

  // Create (or destroy) the vis instance only when the graph container's
  // visibility flips or the component unmounts. Data refreshes update in place.
  useEffect(() => {
    if (!showGraph || !networkRef.current) return;

    const init = () => {
      if (!window.vis?.Network || !networkRef.current) return;
      if (netInstanceRef.current) {
        syncData();
        return;
      }
      const { visNodes, visEdges, textColor } = buildVisData();
      const net = new window.vis.Network(
        networkRef.current,
        { nodes: visNodes, edges: visEdges },
        {
          physics: { enabled: physicsRef.current, stabilization: { iterations: 80 } },
          interaction: { hover: true },
          nodes: { font: { color: textColor, size: 12 }, borderWidth: 2 },
          edges: {
            color: { color: "#6366f1" },
            arrows: { to: { enabled: true, scaleFactor: 0.5 } },
          },
        }
      );
      netInstanceRef.current = net;
      net.on("click", (params: { nodes: string[] }) => {
        const nid = params.nodes[0];
        const found = (nodesRef.current as TopoNode[]).find(
          (n, i) => (n.id || String(i)) === nid
        );
        onNodeClickRef.current(found || null);
      });
    };

    if (window.vis?.Network) {
      init();
    } else {
      const link = document.createElement("link");
      link.rel = "stylesheet";
      link.href = "/js/vis-network.min.css";
      document.head.appendChild(link);

      const script = document.createElement("script");
      script.src = "/js/vis-network.min.js";
      let destroyed = false;
      script.onload = () => {
        if (!destroyed) init();
      };
      document.head.appendChild(script);

      return () => {
        destroyed = true;
        netInstanceRef.current?.destroy();
        netInstanceRef.current = null;
        script.remove();
        link.remove();
      };
    }

    return () => {
      netInstanceRef.current?.destroy();
      netInstanceRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [showGraph]);

  // Live-update the existing instance with fresh data / physics state.
  useEffect(() => {
    if (showGraph) syncData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nodes, edges, physicsEnabled]);

  if (loading) {
    return (
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="text-center">
          <Spinner size="md" className="mx-auto mb-3" />
          <p className="text-muted-foreground text-sm">{t("topology.loading")}</p>
        </div>
      </div>
    );
  }

  if (nodes.length === 0) {
    return (
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="text-center">
          <Network className="w-4 h-4 mx-auto mb-2" />
          <p className="text-muted-foreground text-sm">{t("topology.empty")}</p>
          <p className="text-muted-foreground text-xs mt-1">{t("topology.empty_hint")}</p>
        </div>
      </div>
    );
  }

  return <div ref={networkRef} className="w-full h-full min-h-[500px]" />;
}
