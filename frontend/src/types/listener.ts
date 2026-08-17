export interface Listener {
  id: number;
  name: string;
  scheme: string;
  type: string;
  host: string;
  port: number | string;
  protocol?: string;
  notes?: string;
  enabled?: boolean;
  tags?: string;
  color?: string;
  status?: string;
  dns_domain?: string;
  dns_listen_addr?: string;
  icmp_addr?: string;
  created_at?: string;
  updated_at?: string;
}
