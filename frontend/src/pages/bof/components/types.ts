import { toneForStatus, toneStyles } from "@/lib/ui/statusStyles";

export interface BOFFile {
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  Size?: number | string;
  size?: number;
  Description?: string;
  description?: string;
  Architecture?: string;
  architecture?: string;
  CreatedBy?: string;
  created_by?: string;
  CreatedAt?: string;
  created_at?: string;
}

export interface Execution {
  ID?: string;
  id?: string | number;
  BofName?: string;
  bof_name?: string;
  AgentHostname?: string;
  agent_hostname?: string;
  agent_name?: string;
  Status?: string;
  status?: string;
  Result?: string;
  result?: string;
  Args?: string;
  args?: string;
  CreatedAt?: string;
  created_at?: string;
  Elapsed?: string;
  elapsed?: string;
}

// The server's repo index is a curated list of collections: name, description
// and URL only. There is no backend for per-item ratings, categories,
// architectures, star counts or import state, so the type does not pretend.
export interface RepoItem {
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  Description?: string;
  description?: string;
  URL?: string;
  url?: string;
  Author?: string;
  author?: string;
}

export interface QuickBOF {
  name: string;
  /** i18n key (bof.quick.*) resolved by the consumer — never render this raw. */
  descKey: string;
  arch: string;
  args: string;
}

export const quickBOFLibrary: QuickBOF[] = [
  { name: "adcs_enum", descKey: "bof.quick.adcs_enum", arch: "x64", args: "" },
  { name: "sc_shutdown_elevated", descKey: "bof.quick.sc_shutdown_elevated", arch: "x64", args: "" },
  { name: "netuserenum", descKey: "bof.quick.netuserenum", arch: "x64", args: "/groups" },
  { name: "enumerate-laps", descKey: "bof.quick.enumerate_laps", arch: "x64", args: "" },
  { name: "uptime", descKey: "bof.quick.uptime", arch: "x64", args: "" },
  { name: "env-list", descKey: "bof.quick.env_list", arch: "x64", args: "" },
  { name: "ldap-search", descKey: "bof.quick.ldap_search", arch: "x64", args: "(objectClass=*)" },
  { name: "kerberoast", descKey: "bof.quick.kerberoast", arch: "x64", args: "" },
  { name: "clipboard", descKey: "bof.quick.clipboard", arch: "x64", args: "" },
  { name: "wts_enum", descKey: "bof.quick.wts_enum", arch: "x64", args: "" },
  { name: "window-list", descKey: "bof.quick.window_list", arch: "x64", args: "" },
  { name: "tcp-scan", descKey: "bof.quick.tcp_scan", arch: "x64", args: "10.0.0.1 80-443" },
];

export const getStatusColor = (status: string) => {
  const { bg, text } = toneStyles[toneForStatus((status || "").toLowerCase())];
  return `${bg} ${text}`;
};
