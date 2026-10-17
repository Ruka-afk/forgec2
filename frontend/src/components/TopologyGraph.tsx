
import { useEffect, useRef, useMemo, useState } from "react";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { Network } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { TopoNode, TopoData } from "@/types/topology";

/** A blocked/offline fetch of the vis-network bundle fires neither onload nor
 *  onerror behind some proxies; without a watchdog the container stayed blank. */
const SCRIPT_LOAD_TIMEOUT_MS = 15_000;

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
  /** Auto-discovered network view data (GET /api/topology/network). */
  netData?: TopoData;
  useMeshSource: boolean;
  useNetSource?: boolean;
  physicsEnabled: boolean;
  loading: boolean;
  onNodeClick: (node: TopoNode | null) => void;
}

// Per-group node palette for the auto-discovered network view.
const NET_GROUP_COLORS: Record<string, string> = {
  "agent-online": "#22c55e",
  "agent-offline": "#64748b",
  "host-lateral": "#ef4444",
  "host-discovered": "#38bdf8",
};

// Relation-kind edge styling; kinds only exist on the network source.
const NET_EDGE_STYLE: Record<string, { color: string; dashes?: number[] }> = {
  p2p: { color: "#eab308", dashes: [5, 5] },
  proxy: { color: "#f97316", dashes: [2, 3] },
  discovered: { color: "#475569", dashes: [8, 6] },
};

export default function TopologyGraph({
  data,
  meshData,
  netData,
  useMeshSource,
  useNetSource = false,
  physicsEnabled,
  loading,
  onNodeClick,
}: TopologyGraphProps) {
  const { t } = useI18n();
  const networkRef = useRef<HTMLDivElement>(null);
  const netInstanceRef = useRef<InstanceType<NonNullable<typeof window.vis>["Network"]> | null>(null);
  const [scriptFailed, setScriptFailed] = useState(false);
  const [loadAttempt, setLoadAttempt] = useState(0);

  const nodes = useMemo(() => (useNetSource
    ? (netData?.nodes || [])
    : useMeshSource
      ? (meshData?.nodes || [])
      : (data?.nodes || [])),
  [useNetSource, useMeshSource, netData?.nodes, meshData?.nodes, data?.nodes]);
  const edges = useMemo(() => (useNetSource
    ? (netData?.edges || [])
    : useMeshSource
      ? (meshData?.edges || [])
      : (data?.edges || [])),
  [useNetSource, useMeshSource, netData?.edges, meshData?.edges, data?.edges]);

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

  // Ref mirrors for callbacks used inside the vis init closure.
  const useNetSourceRef = useRef(useNetSource);
  useNetSourceRef.current = useNetSource;

  // Theme foreground is read once per mount, not on every data sync:
  // getComputedStyle forces a style reflow each call.
  const textColorRef = useRef<string | null>(null);
  const readTextColor = () => {
    if (textColorRef.current == null) {
      textColorRef.current =
        getComputedStyle(document.documentElement)
          .getPropertyValue("--foreground")
          .trim() || "#e2e8f0";
    }
    return textColorRef.current;
  };

  const buildVisData = () => {
    const textColor = readTextColor();
    const currentNodes = nodesRef.current as TopoNode[];
    const visNodes = currentNodes.map((n, i) => ({
      id: n.id || String(i),
      label: n.label || n.id || "?",
      group: n.group || "default",
      title: n.title,
      // Network view: per-group colors so controlled vs discovered vs
      // lateral-touched hosts are readable at a glance.
      color: useNetSourceRef.current
        ? {
            background: NET_GROUP_COLORS[n.group || ""] || "#64748b",
            border: NET_GROUP_COLORS[n.group || ""] || "#64748b",
          }
        : undefined,
      shape: useNetSourceRef.current && n.group?.startsWith("host") ? "dot" : undefined,
      size: useNetSourceRef.current && n.group?.startsWith("host") ? 14 : undefined,
    }));
    const visEdges = edgesRef.current.map((e, i) => {
      const style = useNetSourceRef.current && e.kind ? NET_EDGE_STYLE[e.kind] : undefined;
      return {
        id: i,
        from: e.from,
        to: e.to,
        color: style?.color ? { color: style.color } : undefined,
        dashes: style?.dashes,
      };
    });
    return { visNodes, visEdges, textColor };
  };

  // Skip setData when the id sets are unchanged: every 10s topology tick
  // otherwise rebuilds the whole vis dataset even with zero changes.
  const dataSigRef = useRef("");
  const syncData = () => {
    const net = netInstanceRef.current;
    if (!net) return;
    const sig =
      (nodesRef.current as TopoNode[]).map((n, i) => n.id || String(i)).join(",") +
      "|" +
      edgesRef.current.map((e) => `${e.from}>${e.to}`).join(",");
    if (sig === dataSigRef.current) {
      net.setOptions({ physics: { enabled: physicsRef.current } });
      return;
    }
    dataSigRef.current = sig;
    const { visNodes, visEdges } = buildVisData();
    net.setData({ nodes: visNodes, edges: visEdges });
    net.setOptions({ physics: { enabled: physicsRef.current } });
  };

  const showGraph = !loading && nodes.length > 0;

  // Create (or destroy) the vis instance only when the graph container flips
  // visibility, the operator retries a failed bundle load, or the component
  // unmounts. Data refreshes update in place.
  //
  // scriptFailed is a dependency because the fallback branch replaces the
  // graph container: while it is shown, networkRef points at nothing, and a
  // retry must first re-mount the container (retry clears scriptFailed) before
  // this effect can own it again.
  useEffect(() => {
    if (!showGraph || scriptFailed || !networkRef.current) return;

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
      let watchdog = 0;
      const fail = () => {
        if (destroyed) return;
        destroyed = true;
        window.clearTimeout(watchdog);
        script.remove();
        link.remove();
        setScriptFailed(true);
      };
      watchdog = window.setTimeout(fail, SCRIPT_LOAD_TIMEOUT_MS);
      script.onload = () => {
        if (destroyed) return;
        window.clearTimeout(watchdog);
        init();
      };
      script.onerror = () => fail();
      document.head.appendChild(script);

      return () => {
        destroyed = true;
        window.clearTimeout(watchdog);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps -- re-runs only on visibility or a retry (loadAttempt); data updates are the sync effect's job and the loader reads refs by design.
  }, [showGraph, scriptFailed, loadAttempt]);

  // Live-update the existing instance with fresh data / physics state.
  useEffect(() => {
    if (showGraph) syncData();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- syncData is a fresh closure over the ref-held instance; data/physics identity is exactly when a re-sync is wanted.
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
          <Network className="size-4 mx-auto mb-2" />
          <p className="text-muted-foreground text-sm">{t("topology.empty")}</p>
          <p className="text-muted-foreground text-xs mt-1">{t("topology.empty_hint")}</p>
        </div>
      </div>
    );
  }

  // Bundle fetch failed (blocked, offline, or timed out): say so and offer a
  // retry instead of leaving an empty canvas that never explains itself.
  if (scriptFailed) {
    return (
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="text-center max-w-sm px-4">
          <Network className="size-4 mx-auto mb-2 text-destructive-ink" />
          <p className="text-muted-foreground text-sm">{t("topology.graph_load_failed")}</p>
          <p className="text-muted-foreground text-xs mt-1">{t("topology.graph_load_failed_hint")}</p>
          <Button
            variant="outline"
            size="sm"
            className="mt-3"
            onClick={() => {
              // Order matters: clearing the flag re-mounts the graph container,
              // and the bump makes the effect re-run even for a double retry.
              setScriptFailed(false);
              setLoadAttempt((n) => n + 1);
            }}
          >
            {t("common.retry")}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <>
      <div
        ref={networkRef}
        className="w-full h-full min-h-[500px]"
        role="img"
        aria-label={`${t("topology.graph_alt")}: ${nodes.length} nodes, ${edges.length} links`}
      />
      {/* Screen-reader fallback: the canvas graph exposes no semantics. */}
      <table className="sr-only">
        <caption>{t("topology.graph_alt")}</caption>
        <tbody>
          {nodes.map((n, i) => (
            <tr key={n.id || String(i)}>
              <td>{n.label || n.id || "?"}</td>
              <td>{n.group || ""}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
