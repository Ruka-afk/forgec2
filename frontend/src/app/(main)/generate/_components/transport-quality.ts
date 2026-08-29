export type TransportQuality = "core" | "hardened" | "experimental";

interface TransportOption {
  value: string;
  label: string;
  quality: TransportQuality;
}

/** Operator-facing beacon transports with honest quality. */
export const BEACON_TRANSPORTS: TransportOption[] = [
  { value: "http", label: "HTTP(S)", quality: "core" },
  { value: "tcp", label: "TCP", quality: "core" },
  { value: "dns", label: "DNS", quality: "hardened" },
  { value: "wss", label: "WSS", quality: "hardened" },
  { value: "grpc", label: "gRPC", quality: "experimental" },
  { value: "ssh", label: "SSH", quality: "experimental" },
  { value: "icmp", label: "ICMP", quality: "experimental" },
  { value: "mtls", label: "mTLS", quality: "experimental" },
  { value: "h2c", label: "H2C", quality: "experimental" },
  { value: "udp", label: "UDP", quality: "experimental" },
  { value: "quic", label: "QUIC", quality: "experimental" },
];

export function transportQuality(value: string): TransportQuality {
  return BEACON_TRANSPORTS.find((t) => t.value === value)?.quality ?? "experimental";
}

export function isExperimentalTransport(value: string): boolean {
  return transportQuality(value) === "experimental";
}

export function visibleBeaconTransports(showExperimental: boolean): TransportOption[] {
  return BEACON_TRANSPORTS.filter((t) => showExperimental || t.quality !== "experimental");
}

export function qualityLabelKey(quality: TransportQuality): string {
  return `generate.quality_${quality}`;
}
