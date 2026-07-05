export interface SettingsData {
  CurrentUsername?: string;
  current_username?: string;
  CurrentUserRole?: string;
  current_user_role?: string;
  CurrentUserId?: number;
  current_user_id?: number;
  ServerPort?: number;
  server_port?: number;
  ServerAddress?: string;
  server_address?: string;
  LogLevel?: string;
  log_level?: string;
  TLSEnabled?: boolean;
  tls_enabled?: boolean;
  TCPEnabled?: boolean;
  tcp_enabled?: boolean;
  TCPAddr?: string;
  tcp_addr?: string;
  DefaultInterval?: number;
  default_interval?: number;
  DefaultJitter?: number;
  default_jitter?: number;
  DefaultSkipTLS?: boolean;
  default_skip_tls?: boolean;
  DefaultUA?: string;
  default_ua?: string;
  ServerVersion?: string;
  server_version?: string;
  Uptime?: string;
  uptime?: string;
  GoVersion?: string;
  go_version?: string;
  GOOS?: string;
  goos?: string;
  GOARCH?: string;
  goarch?: string;
  Goroutines?: number;
  goroutines?: number;
  TotalAgents?: number;
  total_agents?: number;
  OnlineAgents?: number;
  online_agents?: number;
  TotalListeners?: number;
  total_listeners?: number;
  TotalTasks?: number;
  total_tasks?: number;
  TotalCredentials?: number;
  total_credentials?: number;
  TotalAudits?: number;
  total_audits?: number;
  DatabaseSize?: number;
  database_size?: number;
  AllocMem?: number;
  alloc_mem?: number;
  TotalAllocMem?: number;
  total_alloc_mem?: number;
  NumCPU?: number;
  num_cpu?: number;
  DataDir?: string;
  data_dir?: string;
  JWTMasked?: string;
  jwt_masked?: string;
  TOTPEnabled?: boolean;
  totp_enabled?: string;
  MalleableEnabled?: boolean;
  malleable_enabled?: boolean;
  MalleableStatus?: number;
  malleable_status?: number;
  MalleableCT?: string;
  malleable_ct?: string;
  MalleablePrepend?: string;
  malleable_prepend?: string;
  MalleableAppend?: string;
  malleable_append?: string;
  OfflineThreshold?: number;
  offline_threshold?: number;
  SessionMaxAge?: number;
  session_max_age?: number;
  CleanupRetention?: number;
  cleanup_retention?: number;
  DatabasePath?: string;
  database_path?: string;
}

export interface AgentForm {
  interval: number;
  jitter: number;
  skip_tls: boolean;
  user_agent: string;
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
