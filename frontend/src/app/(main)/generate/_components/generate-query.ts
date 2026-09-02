import { defaultPayloadFormat, isPayloadFormat, type PayloadFormat } from "./generate-format";

export function formatFromOs(os?: string | null): PayloadFormat | undefined {
  switch ((os || "").toLowerCase()) {
    case "windows":
    case "win":
    case "win32":
      return "exe";
    case "linux":
      return "linux";
    case "darwin":
    case "macos":
    case "osx":
      return "macos";
    default:
      return undefined;
  }
}

export interface GenerateQuery {
  listenerId?: string;
  format?: PayloadFormat;
  os?: string;
  arch?: string;
}

export function parseGenerateQuery(search: string): GenerateQuery {
  const raw = search.startsWith("?") ? search.slice(1) : search;
  const params = new URLSearchParams(raw);
  const out: GenerateQuery = {};
  const lid = params.get("listener_id")?.trim();
  if (lid) out.listenerId = lid;
  const formatRaw = params.get("format");
  if (formatRaw && isPayloadFormat(formatRaw)) out.format = formatRaw;
  const os = params.get("os")?.trim();
  if (os) out.os = os;
  const arch = params.get("arch")?.trim();
  if (arch) out.arch = arch;
  if (!out.format && os) {
    const fromOs = formatFromOs(os);
    if (fromOs) out.format = fromOs;
  }
  return out;
}

export function rebuildPayloadHref(agent: {
  listener_id?: string | number | null;
  os?: string | null;
  arch?: string | null;
}): string {
  const params = new URLSearchParams();
  const lid = agent.listener_id != null ? String(agent.listener_id).trim() : "";
  if (lid && lid !== "0") params.set("listener_id", lid);
  const os = (agent.os || "").trim();
  if (os) params.set("os", os);
  const arch = (agent.arch || "").trim();
  if (arch) params.set("arch", arch);
  const format = formatFromOs(os);
  if (format) params.set("format", format);
  const q = params.toString();
  return q ? `/generate?${q}` : "/generate";
}

export function resolvedGenerateFormat(search: string, fallback?: string | null): PayloadFormat {
  return parseGenerateQuery(search).format || defaultPayloadFormat(fallback);
}
