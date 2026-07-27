export interface Screenshot {
  id: string;
  agent_id: string;
  filename: string;
  path: string;
  created_at: string;
}

export interface Resolution {
  w: number;
  h: number;
}
