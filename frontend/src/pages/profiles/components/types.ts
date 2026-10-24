export interface MalleableForm {
  enabled: boolean;
  status_code: number;
  content_type: string;
  headers_text: string;
  prepend: string;
  append: string;
}

export interface AgentProfile {
  name: string;
  description: string;
  user_agent: string;
  beacon_uri: string;
  method: string;
  headers: Record<string, string>;
  sleep: number;
  jitter: number;
  // v2 highly-customizable fields (all optional, backward compatible)
  beacon_uris?: string[];
  uris?: string[];
  prepend?: string;
  append?: string;
  request_prepend?: string;
  request_append?: string;
  request_headers?: Record<string, string>;
  request_header_pool?: string[];
  server_output?: string;
  client_metadata?: string;
  client_id?: string;
  content_length_jitter?: number;
  parameter?: string;
  placements?: string;
  user_agents?: string[];
  jitter_uri?: boolean;
  jitter_parameter?: boolean;
  parameter_names?: string[];
  work_start?: string;
  work_end?: string;
  work_tz?: string;
  /** Editable CS source, returned by GET /api/generate/profiles. */
  source?: string;
  /** Embedded profiles are read-only; Save As is required to change them. */
  builtin?: boolean;
  /** Local-only marker for an unsaved new/duplicated/imported draft. */
  draft?: boolean;
}

export interface ProfileDiagnostic {
  line: number;
  column: number;
  severity: "error" | "warning";
  code: string;
  message: string;
}

export interface CompiledProfileSummary {
  name?: string;
  method?: string;
  uris?: string[];
  sleep?: number;
  jitter?: number;
  user_agent?: string;
  headers?: Record<string, string>;
  client_metadata?: string;
  client_id?: string;
  server_output?: string;
  parameter?: string;
  content_length_jitter?: number;
  jitter_uri?: boolean;
  jitter_parameter?: boolean;
  placements?: { target: string; chain?: string }[];
  ignored?: string[];
}

export interface ProfileValidationData {
  valid?: boolean;
  profile?: AgentProfile;
  compiled?: CompiledProfileSummary | null;
  diagnostics?: ProfileDiagnostic[];
  warnings?: string[];
  wire?: Record<string, string>;
  sample?: { input: string; encoded: string };
}
