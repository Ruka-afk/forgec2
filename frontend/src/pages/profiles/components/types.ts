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
  server_output?: string;
  client_metadata?: string;
  client_id?: string;
  content_length_jitter?: number;
  parameter?: string;
  placements?: string;
  user_agents?: string[];
  jitter_uri?: boolean;
  parameter_names?: string[];
  work_start?: string;
  work_end?: string;
  work_tz?: string;
}

export interface ActiveMalleableConfig {
  malleable_enabled: boolean;
  malleable_profile: string;
  status_code: number;
  content_type: string;
  headers: Record<string, string>;
  user_agent: string;
  jitter: number;
  interval: number;
  prepend: string;
  append: string;
}

export const commonUAs = [
  { label: "Chrome 120 (Windows)", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36" },
  { label: "Edge 120 (Windows)", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0.0 Safari/537.36 Edg/120.0.0.0" },
  { label: "Firefox 121 (Windows)", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0" },
  { label: "Cloudflare Health Check", value: "Mozilla/5.0 (compatible; Cloudflare-Health-Checks/1.0; +https://www.cloudflare.com/)" },
  { label: "GitHub Hookshot", value: "GitHub-Hookshot/abcd1234" },
  { label: "Office 365", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0.0 Safari/537.36 OPR/106.0.0.0" },
  { label: "Microsoft Teams", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Teams/1.6.00.27573" },
  { label: "Slack", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0.0 Safari/537.36 Slack/4.36.0" },
  { label: "Zoom", value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Zoom/5.17.5" },
  { label: "Dropbox", value: "DropboxDesktopClient/187.4.6204 (Windows; 10.0; Win64; x64)" },
  { label: "Windows Update", value: "Windows-Update-Agent/10.0.19041.3636" },
  { label: "Safari (macOS)", value: "Mac OS X/10.15.7 (KHTML, like Gecko) Version/17.2 Safari/605.1.15" },
  { label: "Adobe Creative Cloud", value: "Creative Cloud/6.4.0.361 (Windows; x64)" },
  { label: "Linux Browser", value: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36" },
];

export const emptyProfile = (): AgentProfile => ({
  name: "",
  description: "",
  user_agent: commonUAs[0].value,
  beacon_uri: "/api/v1/beacon",
  method: "POST",
  headers: { Accept: "*/*" },
  sleep: 10,
  jitter: 20,
});

export const emptyActiveConfig = (): ActiveMalleableConfig => ({
  malleable_enabled: false,
  malleable_profile: "",
  status_code: 200,
  content_type: "application/json",
  headers: {},
  user_agent: "",
  jitter: 0,
  interval: 0,
  prepend: "",
  append: "",
});

export const emptyMalleableForm = (): MalleableForm => ({
  enabled: false,
  status_code: 200,
  content_type: "application/json",
  headers_text: "",
  prepend: "",
  append: "",
});
