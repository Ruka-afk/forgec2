export interface Beacon {
  id?: string;
  hostname?: string;
  username?: string;
  ip?: string;
  os?: string;
  arch?: string;
  status?: string;
  last_seen?: string;
  integrity?: string;
  elevated?: boolean;
  notes?: string;
  active_window?: string;
  version?: string;
  parent_id?: string;
  public_ip?: string;
  domain?: string;
  country?: string;
  current_interval?: number;
  current_jitter?: number;
  pid?: number;
  process_name?: string;
  created_at?: string;
}

export interface BulkResult {
  id: string;
  type: string;
  command: string;
  agent_ids: string[];
  task_count: number;
  failed: string[];
  created_at: string;
}

export interface Tag {
  id: string;
  name: string;
  color: string;
}

export const COMMAND_TYPES = [
  { value: "shell", label: "Shell" },
  { value: "ps", label: "Process List" },
  { value: "ls", label: "List Directory" },
  { value: "screenshot", label: "Screenshot" },
  { value: "sleep", label: "Sleep" },
  { value: "kill", label: "Kill" },
  { value: "uninstall", label: "Uninstall" },
];

export function avatarInitial(hostname: string) {
  return (hostname || "?").charAt(0).toUpperCase();
}

export function avatarColor(hostname: string) {
  const colors = ["bg-indigo-500", "bg-emerald-500", "bg-amber-500", "bg-rose-500", "bg-cyan-500"];
  let h = 0;
  for (let i = 0; i < hostname.length; i++) h = hostname.charCodeAt(i) + ((h << 5) - h);
  return colors[Math.abs(h) % colors.length];
}

export function copyToClipboard(text: string, key: string, setCopied: (k: string) => void) {
  navigator.clipboard.writeText(text).then(() => { setCopied(key); setTimeout(() => setCopied(""), 1500); }).catch(() => {});
}

export function formatUptime(dateStr: string): string {
  if (!dateStr) return "";
  const diff = Date.now() - new Date(dateStr).getTime();
  if (diff < 0) return "";
  const d = Math.floor(diff / 86400000);
  const h = Math.floor((diff % 86400000) / 3600000);
  const m = Math.floor((diff % 3600000) / 60000);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}
