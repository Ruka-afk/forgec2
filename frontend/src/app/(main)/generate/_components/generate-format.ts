export const PAYLOAD_FORMATS = ["exe", "dll", "ps1", "linux", "macos", "oneliner"] as const;
export type PayloadFormat = (typeof PAYLOAD_FORMATS)[number];

export const PAYLOAD_FORMAT_LABEL: Record<PayloadFormat, string> = {
  exe: "generate.format_exe",
  dll: "generate.format_dll",
  ps1: "generate.format_ps1",
  linux: "generate.format_linux",
  macos: "generate.format_macos",
  oneliner: "generate.format_oneliner",
};

export function isPayloadFormat(v: string): v is PayloadFormat {
  return (PAYLOAD_FORMATS as readonly string[]).includes(v);
}

export function defaultPayloadFormat(raw?: string | null): PayloadFormat {
  return raw && isPayloadFormat(raw) ? raw : "exe";
}
