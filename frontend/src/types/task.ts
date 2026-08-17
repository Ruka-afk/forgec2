export interface Task {
  id: number;
  agent_id: string;
  type: string;
  command: string;
  shell?: string;
  path?: string;
  status: string;
  result: string;
  error: string;
  progress?: number;
  total_bytes?: number;
  transferred?: number;
  created_by: string;
  approved_by?: string;
  approved_at?: string;
  claimed_by: string;
  claimed_at: string;
  acknowledged_at?: string;
  callback_url?: string;
  callback_method?: string;
  callback_sent?: boolean;
  created_at: string;
  updated_at: string;
}
