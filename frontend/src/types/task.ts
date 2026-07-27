export interface Task {
  id: number;
  agent_id: string;
  type: string;
  command: string;
  status: string;
  result: string;
  error: string;
  created_by: string;
  claimed_by: string;
  claimed_at: string;
  created_at: string;
  updated_at: string;
}
