import type { Listener } from "./listener";

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
  interval: string;
  jitter: string;
  ua: string;
  proxy: string;
  failover: string;
  crypto_key: string;
  profile: string;
}

export interface EXEForm {
  filename: string;
  persist: boolean;
  skip_tls: boolean;
  evasion: boolean;
  obfuscate: boolean;
  domain_front: string;
  p2p_mode: string;
  p2p_parent: string;
  p2p_listen_addr: string;
  dns_domain: string;
  dns_server: string;
}

export interface PS1Form {
  persist: boolean;
  skip_tls: boolean;
}

export interface LinuxForm {
  filename: string;
  persist: boolean;
  skip_tls: boolean;
  obfuscate: boolean;
  domain_front: string;
}

export interface MacOSForm {
  filename: string;
  persist: boolean;
  skip_tls: boolean;
  obfuscate: boolean;
  domain_front: string;
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

export interface BusyState {
  exe: boolean;
  ps1: boolean;
  linux: boolean;
  macos: boolean;
  stager: boolean;
  stager_linux: boolean;
  shellcode: boolean;
  donut: boolean;

  oneliner: boolean;
}

export interface Results {
  exe: string;
  ps1: string;
  linux: string;
  macos: string;
  stager: string;
  stager_linux: string;
  shellcode: string;
  donut: string;

  oneliner: string;
  _ps1Code?: string;
  _ps1Original?: number;
  _ps1Obfuscated?: number;
  _onelinerData?: OneLinerData;
}
