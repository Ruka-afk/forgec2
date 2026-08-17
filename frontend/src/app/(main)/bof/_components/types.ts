"use client";

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

export interface BOFLibraryItem {
  id?: number | string;
  name?: string;
  description?: string;
  arch?: string;
  author?: string;
  size?: number;
  created_by?: string;
  created_at?: string;
}

export interface Execution {
  ID?: string;
  id?: string;
  BofName?: string;
  bof_name?: string;
  AgentHostname?: string;
  agent_hostname?: string;
  Status?: string;
  status?: string;
  Result?: string;
  result?: string;
  Args?: string;
  args?: string;
  CreatedAt?: string;
  created_at?: string;
}

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
  Stars?: number;
  stars?: number;
  Downloads?: number;
  downloads?: number;
  Category?: string;
  category?: string;
  Architecture?: string;
  architecture?: string;
  Rating?: number;
  rating?: number;
  Reviews?: number;
  reviews?: number;
  Imported?: boolean;
  imported?: boolean;
}

export interface QuickBOF {
  name: string;
  desc: string;
  arch: string;
  args: string;
}

export const quickBOFLibrary: QuickBOF[] = [
  { name: "adcs_enum", desc: "Enumerate AD CS templates and certificate authorities", arch: "x64", args: "" },
  { name: "sc_shutdown_elevated", desc: "Shutdown system with elevated privileges", arch: "x64", args: "" },
  { name: "netuserenum", desc: "Enumerate domain users via various methods", arch: "x64", args: "/groups" },
  { name: "enumerate-laps", desc: "Enumerate LAPS passwords from AD", arch: "x64", args: "" },
  { name: "uptime", desc: "Get system uptime information", arch: "x64", args: "" },
  { name: "env-list", desc: "List environment variables", arch: "x64", args: "" },
  { name: "ldap-search", desc: "Perform LDAP searches from beacon", arch: "x64", args: "(objectClass=*)" },
  { name: "kerberoast", desc: "Request TGS tickets for kerberoasting", arch: "x64", args: "" },
  { name: "clipboard", desc: "Monitor clipboard contents", arch: "x64", args: "" },
  { name: "wts_enum", desc: "Enumerate Remote Desktop sessions", arch: "x64", args: "" },
  { name: "window-list", desc: "List visible windows on desktop", arch: "x64", args: "" },
  { name: "tcp-scan", desc: "Internal TCP port scanner", arch: "x64", args: "10.0.0.1 80-443" },
];

export const getStatusColor = (status: string) => {
  const { bg, text } = toneStyles[toneForStatus((status || "").toLowerCase())];
  return `${bg} ${text}`;
};
