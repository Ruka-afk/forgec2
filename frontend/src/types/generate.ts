import type { Listener } from "./listener";
import type { ReactNode } from "react";

export interface ProfilePreset {
  name: string;
  description?: string;
  user_agent?: string;
  sleep?: number;
  jitter?: number;
}

export interface OneLinerType {
  name: string;
  desc: string;
  command: string;
}

export interface OneLinerData {
  download_url: string;
  types: OneLinerType[];
}

export interface SharedState {
  listener_id: string;
  c2_url: string;
  protocol: string;
  beacon_transport: string;
  interval: string;
  jitter: string;
  ua: string;
  proxy: string;
  failover: string;
  crypto_key: string;
  beacon_key: string;
  profile: string;
  dns_doh_url: string;
  dns_dot_addr: string;
  ssh_user: string;
  ssh_password: string;
  ssh_key: string;
  ssh_host_key: string;
}

export interface BinaryForm {
  filename: string;
  persist: boolean;
  skip_tls: boolean;
  evasion: boolean;
  ghost_mode: boolean;
  obfuscate: boolean;
  arch: string;
  domain_front: string;
  p2p_mode: string;
  p2p_parent: string;
  p2p_listen_addr: string;
  dns_domain: string;
  dns_server: string;
  working_start: string;
  working_end: string;
  working_tz: string;
}

export interface UnixForm {
  filename: string;
  persist: boolean;
  skip_tls: boolean;
  obfuscate: boolean;
  domain_front: string;
  working_start: string;
  working_end: string;
  working_tz: string;
}

export interface PS1Form {
  filename: string;
  persist: boolean;
  skip_tls: boolean;
}

export interface StagerForm {
  filename: string;
  skip_tls: boolean;
}

export interface ShellcodeForm {
  command: string;
  filename: string;
}

export interface DonutForm {
  arch: string;
  class: string;
  method: string;
  args: string;
  filename: string;
  assembly: File | null;
}

export interface OneLinerForm {
  payload_type: string;
  beacon_time: string;
  jitter: string;
  skip_tls: boolean;
  persist: boolean;
  listener_id: string;
  c2_url: string;
  protocol: string;
}

export type BinaryVariant = "exe" | "dll";
export type UnixVariant = "linux" | "macos";
export type StagerVariant = "windows" | "linux";

export interface GenerateResult {
  error?: string;
  success?: boolean;
}

export interface GenerateState {
  busy: boolean;
  result: ReactNode;
}

export interface PS1Result extends GenerateResult {
  code?: string;
  original_length?: number;
  obfuscated_len?: number;
}

export interface OneLinerResult extends GenerateResult {
  data?: OneLinerData;
}

export interface PayloadForms {
  exe: BinaryForm;
  dll: BinaryForm;
  ps1: PS1Form;
  linux: UnixForm;
  macos: UnixForm;
  stager: StagerForm;
  stager_linux: StagerForm;
  shellcode: ShellcodeForm;
  donut: DonutForm;
  oneliner: OneLinerForm;
}

export interface PayloadStates {
  exe: GenerateState;
  dll: GenerateState;
  ps1: GenerateState;
  linux: GenerateState;
  macos: GenerateState;
  stager: GenerateState;
  stager_linux: GenerateState;
  shellcode: GenerateState;
  donut: GenerateState;
  oneliner: GenerateState;
}

export interface PayloadExtras {
  ps1?: PS1Result;
  oneliner?: OneLinerResult;
}

export type PayloadKey = keyof PayloadForms;

export const DEFAULT_BINARY_FORM: BinaryForm = {
  filename: "", persist: false, skip_tls: false, evasion: false, ghost_mode: false, obfuscate: false,
  arch: "amd64", domain_front: "", p2p_mode: "", p2p_parent: "",
  p2p_listen_addr: "", dns_domain: "", dns_server: "",
  working_start: "", working_end: "", working_tz: "",
};

export const DEFAULT_UNIX_FORM: UnixForm = {
  filename: "forge_implant", persist: false, skip_tls: false, obfuscate: false, domain_front: "",
  working_start: "", working_end: "", working_tz: "",
};

export const DEFAULT_PS1_FORM: PS1Form = { filename: "", persist: false, skip_tls: false };
export const DEFAULT_STAGER_FORM: StagerForm = { filename: "", skip_tls: false };
export const DEFAULT_SHELLCODE_FORM: ShellcodeForm = { command: "powershell -NoP -EP Bypass -Enc ...", filename: "shellcode.bin" };
export const DEFAULT_DONUT_FORM: DonutForm = { arch: "amd64", class: "", method: "", args: "", filename: "donut_loader.bin", assembly: null };
export const DEFAULT_ONELINER_FORM: OneLinerForm = { payload_type: "exe", beacon_time: "5", jitter: "10", skip_tls: false, persist: false, listener_id: "", c2_url: "", protocol: "http" };

export function createDefaultForms(): PayloadForms {
  return {
    exe: { ...DEFAULT_BINARY_FORM, filename: "forge_agent.exe" },
    dll: { ...DEFAULT_BINARY_FORM, filename: "forge_agent.dll" },
    ps1: { ...DEFAULT_PS1_FORM },
    linux: { ...DEFAULT_UNIX_FORM },
    macos: { ...DEFAULT_UNIX_FORM },
    stager: { ...DEFAULT_STAGER_FORM, filename: "stager.exe" },
    stager_linux: { ...DEFAULT_STAGER_FORM, filename: "stager" },
    shellcode: { ...DEFAULT_SHELLCODE_FORM },
    donut: { ...DEFAULT_DONUT_FORM },
    oneliner: { ...DEFAULT_ONELINER_FORM },
  };
}

export function createDefaultStates(): PayloadStates {
  const keys: PayloadKey[] = ["exe", "dll", "ps1", "linux", "macos", "stager", "stager_linux", "shellcode", "donut", "oneliner"];
  const states = {} as PayloadStates;
  for (const k of keys) states[k] = { busy: false, result: "" };
  return states;
}

export interface BuildHistoryEntry {
  id: number;
  platform: string;
  format: string;
  c2_url: string;
  listener_id: number;
  filename: string;
  user: string;
  status: string;
  error: string;
  output_path: string;
  created_at: string;
}

export type { Listener };
