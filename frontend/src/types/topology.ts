export interface TopoNode {
  id?: string;
  label?: string;
  group?: string;
  title?: string;
  p2p_mode?: string;
  peer_count?: number;
}

export interface TopoEdge {
  from: string;
  to: string;
  /** Relation kind — only the auto-discovered network view sets it. */
  kind?: "p2p" | "proxy" | "discovered";
}

export interface TopoData {
  nodes?: TopoNode[];
  edges?: TopoEdge[];
}

/** Response of GET /api/topology/network */
export interface NetTopologyData {
  nodes: TopoNode[];
  edges: TopoEdge[];
  stats: {
    agents: number;
    online: number;
    hosts: number;
    hosts_lateral: number;
    hosts_discovered: number;
    merged_into_agent: number;
  };
}
