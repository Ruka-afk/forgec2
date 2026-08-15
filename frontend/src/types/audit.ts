export interface AuditLog {
  id?: string;
  timestamp?: string;
  username?: string;
  action?: string;
  resource?: string;
  target?: string;
  status?: string;
  details?: string;
  ip?: string;
  severity?: string;
  agent_id?: string;
}
