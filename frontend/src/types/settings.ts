export interface SettingsData {
  current_username?: string;
  current_user_role?: string;
  current_user_id?: number;
  server_port?: number;
  server_address?: string;
  log_level?: string;
  tls_enabled?: boolean;
  tcp_enabled?: boolean;
  tcp_addr?: string;
  default_interval?: number;
  default_jitter?: number;
  default_skip_tls?: boolean;
  default_ua?: string;
  server_version?: string;
  uptime?: string;
  go_version?: string;
  goos?: string;
  goarch?: string;
  goroutines?: number;
  total_agents?: number;
  online_agents?: number;
  total_listeners?: number;
  total_tasks?: number;
  total_credentials?: number;
  total_audits?: number;
  database_size?: number;
  alloc_mem?: number;
  total_alloc_mem?: number;
  num_cpu?: number;
  data_dir?: string;
  jwt_masked?: string;
  totp_enabled?: string;
  malleable_enabled?: boolean;
  malleable_status?: number;
  malleable_ct?: string;
  malleable_prepend?: string;
  malleable_append?: string;
  offline_threshold?: number;
  session_max_age?: number;
  cleanup_retention?: number;
  database_path?: string;
  working_start?: string;
  working_end?: string;
  working_tz?: string;
}

export interface AgentForm {
  interval: number;
  jitter: number;
  skip_tls: boolean;
  user_agent: string;
  working_start: string;
  working_end: string;
  working_tz: string;
}

export interface ServerForm {
  log_level: string;
  tcp_enabled: boolean;
  tcp_addr: string;
  offline_threshold: number;
  session_max_age: number;
  cleanup_retention: number;
}

export interface MalleableForm {
  enabled: boolean;
  status_code: number;
  content_type: string;
  headers_text: string;
  prepend: string;
  append: string;
}

export interface PasswordForm {
  current: string;
  next: string;
  confirm: string;
}

export interface PurgeDays {
  tasks: string;
  audit: string;
}
