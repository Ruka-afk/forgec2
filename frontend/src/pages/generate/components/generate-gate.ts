/** Payload generate is only valid when a listener is selected. */
export function canGenerateFromListener(listenerId?: string | null): boolean {
  return Boolean(String(listenerId ?? "").trim());
}

export type TransportFamily = "http" | "tcp" | "dns" | "icmp" | "udp" | "quic" | "other";

export function transportFamily(schemeOrTransport: string): TransportFamily {
  const s = (schemeOrTransport || "").toLowerCase().replace(/:$/, "");
  if (["http", "https", "h2c", "wss", "ws", "grpc", "grpcs", "mtls"].includes(s)) return "http";
  if (s === "tcp" || s === "tls") return "tcp";
  if (s === "dns") return "dns";
  if (s === "icmp") return "icmp";
  if (s === "udp") return "udp";
  if (s === "quic") return "quic";
  return "other";
}

export function transportFromListenerScheme(scheme: string): { transport: string; protocol: string } {
  const s = (scheme || "http").toLowerCase();
  if (s === "tcp" || s === "tls") return { transport: "tcp", protocol: "tcp" };
  if (s === "dns") return { transport: "dns", protocol: "dns" };
  if (s === "grpc" || s === "grpcs") return { transport: "grpc", protocol: "http" };
  if (s === "ssh") return { transport: "ssh", protocol: "http" };
  if (s === "wss" || s === "ws") return { transport: "wss", protocol: "http" };
  if (s === "icmp") return { transport: "icmp", protocol: "icmp" };
  if (s === "mtls") return { transport: "mtls", protocol: "http" };
  if (s === "h2c") return { transport: "h2c", protocol: "http" };
  if (s === "udp") return { transport: "udp", protocol: "udp" };
  if (s === "quic") return { transport: "quic", protocol: "quic" };
  return { transport: "http", protocol: "http" };
}

/** Scheme baked into the implant C2 URL for the selected beacon transport. */
export function schemeForTransport(listenerScheme: string, beaconTransport: string): string {
  const t = (beaconTransport || "").toLowerCase();
  const ls = (listenerScheme || "http").toLowerCase();
  if (t === "udp" || t === "quic" || t === "dns" || t === "icmp") return t;
  if (t === "tcp") return ls === "tls" ? "tls" : "tcp";
  if (t === "wss" || t === "ws") return t;
  if (ls === "https" || ls === "http" || ls === "h2c") return ls;
  if (transportFamily(t) === transportFamily(ls) && ls) return ls;
  return t || ls || "http";
}

export function composeC2URL(opts: {
  scheme: string;
  host: string;
  port: string | number;
  failover?: string;
}): string {
  const port = String(opts.port ?? "").trim();
  const host = (opts.host || "").trim();
  const scheme = (opts.scheme || "http").replace(/:$/, "");
  let url = `${scheme}://${host}`;
  if (port) url += `:${port}`;
  const fo = (opts.failover || "").trim();
  return fo ? `${url},${fo}` : url;
}

export function failoverHasTransport(failover: string, transport: string): boolean {
  const family = transportFamily(transport);
  if (family === "other") return false;
  for (const part of failover.split(",")) {
    const raw = part.trim();
    if (!raw) continue;
    const withScheme = raw.includes("://") ? raw : `http://${raw}`;
    try {
      const u = new URL(withScheme);
      if (transportFamily(u.protocol.replace(":", "")) === family) return true;
    } catch {
      /* ignore */
    }
  }
  return false;
}

export function listenerTransportCompatible(
  listenerScheme?: string | null,
  beaconTransport?: string | null,
  failover = "",
): boolean {
  const transport = (beaconTransport || "http").toLowerCase();
  const scheme = (listenerScheme || "").toLowerCase();
  if (!scheme) return true;
  if (transportFamily(scheme) === transportFamily(transport)) return true;
  return failoverHasTransport(failover, transport);
}

export function canGeneratePayload(opts: {
  listenerId?: string | null;
  listenerScheme?: string | null;
  beaconTransport?: string | null;
  failover?: string;
}): boolean {
  if (!canGenerateFromListener(opts.listenerId)) return false;
  return listenerTransportCompatible(opts.listenerScheme, opts.beaconTransport, opts.failover);
}
