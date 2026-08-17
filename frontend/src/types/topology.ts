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
}

export interface TopoData {
  nodes?: TopoNode[];
  edges?: TopoEdge[];
}
