"use client";

import { useEffect, useRef, useMemo } from "react";
import { Spinner } from "@/components/UI";
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

  const nodes = useMemo(() => useMeshSource
    ? (meshData?.nodes || [])
    : (data?.nodes || []),
  [useMeshSource, meshData?.nodes, data?.nodes]);
  const edges = useMemo(() => useMeshSource
    ? (meshData?.edges || [])
    : (data?.edges || []),
  [useMeshSource, meshData?.edges, data?.edges]);

  useEffect(() => {
    if (!networkRef.current || loading) return;
    if (nodes.length === 0) return;

    const buildVisData = () => {
      const textColor =
        getComputedStyle(document.documentElement)
          .getPropertyValue("--foreground")
          .trim() || "#e2e8f0";
      const visNodes = (nodes as TopoNode[]).map((n, i) => ({
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
          edges: {
            color: { color: "#6366f1" },
            arrows: { to: { enabled: true, scaleFactor: 0.5 } },
          },
        }
      );
      netInstanceRef.current = net;
      net.on("click", (params: { nodes: string[] }) => {
        const nid = params.nodes[0];
        const found = (nodes as TopoNode[]).find(
          (n, i) => (n.id || String(i)) === nid
        );
        onNodeClick(found || null);
      });
    };

    if (window.vis?.Network) {
      init();
      return () => {
        netInstanceRef.current?.destroy();
        netInstanceRef.current = null;
      };
    }

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
  }, [data, meshData, loading, physicsEnabled, useMeshSource, nodes, edges, onNodeClick]);

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
