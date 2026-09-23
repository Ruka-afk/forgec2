
// taskRenderers — pluggable per-task-type result rendering.
//
// Modules can ship their own result view instead of forcing operators to read
// raw text: register a component against the task type and every result
// surface (task rows, detail modals) picks it up automatically. Types without
// a registration fall back to the classic <pre> output, so the registry is
// purely additive and never blocks a new command's results.

import type { ComponentType } from "react";
import { RefreshCw, ShieldAlert, Monitor, Package, Network, Clock } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { parsePsRows } from "@/pages/agents/detail/components/process-snapshot";

interface TaskRendererProps {
  /** Raw result payload exactly as stored on the task row. */
  result: string;
  /** Canonical task type the renderer was selected for. */
  taskType: string;
}

type TaskRenderer = ComponentType<TaskRendererProps>;

const registry = new Map<string, TaskRenderer>();

/**
 * Register a renderer for a canonical task type. Later registrations
 * overwrite earlier ones (useful for operator overrides).
 */
export function registerTaskRenderer(taskType: string, renderer: TaskRenderer): void {
  registry.set(taskType, renderer);
}

/** Look up the renderer for a task type, if any. */
export function getTaskRenderer(taskType: string): TaskRenderer | undefined {
  return registry.get(taskType);
}

/** Test/debug helper: currently registered task types. */
export function registeredRendererTypes(): string[] {
  return [...registry.keys()].sort();
}

// ── hostinfo renderer ───────────────────────────────────────────────────────

interface HostInfoReport {
  category?: string;
  collected_at?: string;
  platform?: string;
  sections?: Record<string, Record<string, unknown>>;
}

function asStr(v: unknown): string {
  if (v === null || v === undefined) return "";
  return String(v);
}

function asRows(v: unknown): Record<string, unknown>[] {
  if (!Array.isArray(v)) return [];
  return v.filter((x): x is Record<string, unknown> => typeof x === "object" && x !== null);
}

/** Literal-key mapper: the i18n scanner counts only t("…") literals. */


function sectionIcon(key: string) {
  switch (key) {
    case "security": return <ShieldAlert className="size-3.5" />;
    case "system": return <Monitor className="size-3.5" />;
    case "software": return <Package className="size-3.5" />;
    case "network": return <Network className="size-3.5" />;
    default: return <Clock className="size-3.5" />;
  }
}

function MiniTable({ rows }: { rows: Record<string, unknown>[] }) {
  if (rows.length === 0) return null;
  const cols = Object.keys(rows[0]);
  return (
    <div className="overflow-x-auto scrollbar-thin">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border text-left text-muted-foreground">
            {cols.map((c) => (
              <th key={c} className="py-1 pr-4 font-medium whitespace-nowrap">{c}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={i} className="border-b border-border/40 last:border-0">
              {cols.map((c) => (
                <td key={c} className="py-1 pr-4 font-mono align-top max-w-[280px] truncate" title={asStr(row[c])}>
                  {asStr(row[c])}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ProtectionBadge({ state }: { state: string }) {
  const tone =
    state === "enabled"
      ? "bg-destructive/10 text-destructive"
      : state === "disabled"
        ? "bg-success/10 text-success"
        : "bg-muted text-muted-foreground";
  return <Badge variant="outline" className={cn("text-(--fs-micro-sm)", tone)}>{state}</Badge>;
}

/**
 * Structured view for `hostinfo` sweeps: grouped sections with mini tables
 * plus protection badges for AV entries. Falls back to the raw payload when
 * the result is not the expected JSON shape.
 */
export function HostInfoResultView({ result }: TaskRendererProps) {
  let report: HostInfoReport;
  try {
    report = JSON.parse(result) as HostInfoReport;
    if (!report || typeof report !== "object" || !report.sections) throw new Error("shape");
  } catch {
    return <pre className="text-xs text-success font-mono overflow-x-auto max-h-96 p-2 whitespace-pre-wrap break-all">{result}</pre>;
  }

  return (
    <div className="space-y-4" data-testid="hostinfo-renderer">
      <div className="flex items-center gap-2 text-(--fs-micro-sm) text-muted-foreground">
        <RefreshCw className="size-3" />
        <span className="font-mono">{asStr(report.platform)}</span>
        <span>·</span>
        <span className="font-mono">{asStr(report.collected_at)}</span>
      </div>
      {Object.entries(report.sections).map(([key, data]) => {
        // A section can legitimately be null (e.g. a failed sweep serializes
        // "network": null); guard before property access or this crashes.
        if (!data || typeof data !== "object" || Array.isArray(data)) return null;
        const av = asRows(data.av_products);
        return (
          <div key={key}>
            <div className="flex items-center gap-1.5 mb-1.5">
              {sectionIcon(key)}
              <span className="text-xs font-semibold">{key}</span>
            </div>
            {typeof data.error === "string" && (
              <p className="text-xs text-destructive">{data.error}</p>
            )}
            {av.length > 0 && (
              <div className="mb-1.5 flex flex-wrap gap-1.5">
                {av.map((a, i) => (
                  <span key={i} className="inline-flex items-center gap-1.5 rounded-lg border border-border px-2 py-0.5 text-xs">
                    <span className="font-medium">{asStr(a.name)}</span>
                    <ProtectionBadge state={asStr(a.protection) || "unknown"} />
                  </span>
                ))}
              </div>
            )}
            {Object.entries(data)
              .filter(([, v]) => Array.isArray(v))
              .map(([k, v]) => {
                const rows = asRows(v);
                if (rows.length === 0) return null;
                return (
                  <div key={k} className="mt-1.5">
                    <MiniTable rows={rows} />
                  </div>
                );
              })}
          </div>
        );
      })}
    </div>
  );
}

registerTaskRenderer("hostinfo", HostInfoResultView);

// ── shared raw fallback ─────────────────────────────────────────────────────

function RawFallback({ result }: { result: string }) {
  return (
    <pre className="text-xs text-success font-mono overflow-x-auto max-h-96 p-2 whitespace-pre-wrap break-all">{result}</pre>
  );
}

function ResultMeta({ count, label }: { count: number; label: string }) {
  return (
    <div className="flex items-center gap-2 mb-2 text-(--fs-micro-sm) text-muted-foreground">
      <Badge variant="secondary" className="px-1.5 py-px text-(--fs-micro) leading-none rounded bg-primary/15 text-primary font-mono">
        {count}
      </Badge>
      <span>{label}</span>
    </div>
  );
}

// ── ps renderer ─────────────────────────────────────────────────────────────

/**
 * Structured process table for `ps` results. Reuses the same free-form row
 * parser the agent detail ProcessSection uses (Format-Table / `ps aux`).
 */
export function PsResultView({ result }: TaskRendererProps) {
  const rows = parsePsRows(result || "", 500);
  if (rows.length === 0) return <RawFallback result={result} />;
  return (
    <div className="space-y-1" data-testid="ps-renderer">
      <ResultMeta count={rows.length} label="processes" />
      <div className="overflow-x-auto scrollbar-thin max-h-72 overflow-y-auto rounded-lg border border-border">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-border text-left text-muted-foreground bg-muted/40">
              <th className="py-1 px-2 font-medium w-20">PID</th>
              <th className="py-1 px-2 font-medium">Name</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, i) => (
              <tr key={`${row.pid}-${i}`} className="border-b border-border/40 last:border-0">
                <td className="py-1 px-2 font-mono align-top">{row.pid}</td>
                <td className="py-1 px-2 font-mono align-top truncate max-w-[320px]" title={row.raw.trim()}>
                  {row.name}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

registerTaskRenderer("ps", PsResultView);

// ── netstat renderer ────────────────────────────────────────────────────────

interface NetstatRow {
  proto: string;
  local: string;
  remote: string;
  state: string;
  pid: string;
}

const NETSTAT_PROTO = /^(tcp|udp|tcpv6|udpv6|ip|icmp)$/i;

/** Best-effort parse of Windows/Linux `netstat` table lines. */
export function parseNetstatRows(text: string, cap = 400): NetstatRow[] {
  const rows: NetstatRow[] = [];
  for (const rawLine of (text || "").split(/\r?\n/)) {
    if (rows.length >= cap) break;
    const line = rawLine.trim();
    if (!line) continue;
    const tokens = line.split(/\s+/);
    if (!NETSTAT_PROTO.test(tokens[0])) continue;
    // Windows: Proto Local Foreign State PID  |  Linux: Proto Recv-Q Send-Q Local Foreign State PID
    if (tokens.length < 4) continue;
    const proto = tokens[0].toLowerCase();
    // Find first token that looks like host:port (local address)
    const localIdx = tokens.findIndex((t, i) => i > 0 && /:\d+|\*:\d+|:\*$/.test(t) && t.includes(":"));
    if (localIdx < 1) continue;
    const local = tokens[localIdx];
    const remote = tokens[localIdx + 1] || "";
    // State may be absent for UDP; PID is trailing numeric
    let state = "";
    let pid = "";
    const rest = tokens.slice(localIdx + 2);
    for (let i = rest.length - 1; i >= 0; i--) {
      if (!pid && /^\d+$/.test(rest[i])) {
        pid = rest[i];
        continue;
      }
      if (!state && !/^\d+$/.test(rest[i])) {
        state = rest[i];
        break;
      }
    }
    if (!pid && /^\d+$/.test(tokens[tokens.length - 1])) pid = tokens[tokens.length - 1];
    if (!state && rest.length > 0 && !/^\d+$/.test(rest[rest.length - 1])) {
      state = rest[rest.length - 1];
    }
    rows.push({ proto, local, remote, state, pid });
  }
  return rows;
}

export function NetstatResultView({ result }: TaskRendererProps) {
  const rows = parseNetstatRows(result || "");
  if (rows.length === 0) return <RawFallback result={result} />;
  const listening = rows.filter((r) => /listen/i.test(r.state)).length;
  return (
    <div className="space-y-1" data-testid="netstat-renderer">
      <div className="flex items-center gap-2 mb-2 text-(--fs-micro-sm) text-muted-foreground">
        <Badge variant="secondary" className="px-1.5 py-px text-(--fs-micro) leading-none rounded bg-primary/15 text-primary font-mono">
          {rows.length}
        </Badge>
        <span>connections</span>
        {listening > 0 && (
          <Badge variant="secondary" className="px-1.5 py-px text-(--fs-micro) leading-none rounded bg-warning/15 text-warning font-mono">
            {listening} listen
          </Badge>
        )}
      </div>
      <div className="overflow-x-auto scrollbar-thin max-h-72 overflow-y-auto rounded-lg border border-border">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-border text-left text-muted-foreground bg-muted/40">
              <th className="py-1 px-2 font-medium w-14">Proto</th>
              <th className="py-1 px-2 font-medium">Local</th>
              <th className="py-1 px-2 font-medium">Remote</th>
              <th className="py-1 px-2 font-medium w-24">State</th>
              <th className="py-1 px-2 font-medium w-16">PID</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, i) => (
              <tr key={`${row.proto}-${row.local}-${row.remote}-${i}`} className="border-b border-border/40 last:border-0">
                <td className="py-1 px-2 font-mono align-top uppercase">{row.proto}</td>
                <td className="py-1 px-2 font-mono align-top truncate max-w-[160px]" title={row.local}>{row.local}</td>
                <td className="py-1 px-2 font-mono align-top truncate max-w-[160px]" title={row.remote}>{row.remote}</td>
                <td className={cn("py-1 px-2 font-mono align-top", /listen/i.test(row.state) ? "text-warning" : "")}>{row.state || "-"}</td>
                <td className="py-1 px-2 font-mono align-top">{row.pid || "-"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

registerTaskRenderer("netstat", NetstatResultView);

// ── creds renderer ──────────────────────────────────────────────────────────

interface CredRow {
  domain: string;
  username: string;
  secret: string;
  kind: "password" | "hash";
}

/**
 * Mirror of the server-side text parsers (mimikatz blocks, SAM lines,
 * simple domain\user:pass). Best-effort: unparsed dumps fall back to raw.
 */
export function parseCredRows(text: string, cap = 200): CredRow[] {
  const raw = text || "";
  const rows: CredRow[] = [];
  const seen = new Set<string>();

  // Pattern 1: mimikatz-style blocks
  const blocks = raw.split(/\n\s*\n/);
  for (const block of blocks) {
    if (rows.length >= cap) break;
    const userM = /(?:Username|User)\s*:\s*(.+)/i.exec(block);
    const domainM = /Domain\s*:\s*(.+)/i.exec(block);
    const ntlmM = /NTLM\s*:\s*([a-fA-F0-9]{32})/.exec(block);
    const sha1M = /SHA1?\s*:\s*([a-fA-F0-9]{40})/i.exec(block);
    const pwM = /Password\s*:\s*(.+?)\s*$/im.exec(block);
    const username = userM ? userM[1].trim() : "";
    const domain = domainM ? domainM[1].trim() : "";
    let secret = "";
    let kind: CredRow["kind"] = "password";
    if (ntlmM) { secret = ntlmM[1]; kind = "hash"; }
    else if (sha1M) { secret = sha1M[1]; kind = "hash"; }
    else if (pwM && pwM[1].trim() && pwM[1].trim() !== "(null)") { secret = pwM[1].trim(); kind = "password"; }
    if (username && username !== "(null)" && secret) {
      const key = `${domain}|${username}|${secret}`;
      if (!seen.has(key)) {
        seen.add(key);
        rows.push({ domain, username, secret, kind });
      }
    }
  }

  // Pattern 2: SAM hash dump user:rid:lm:nt:::
  if (rows.length === 0) {
    for (const line of raw.split(/\r?\n/)) {
      if (rows.length >= cap) break;
      const m = /^([^\s:]+):(\d+):([a-fA-F0-9]{32}):([a-fA-F0-9]{32}):::/i.exec(line.trim());
      if (m) {
        const key = `sam|${m[1]}|${m[4]}`;
        if (!seen.has(key)) {
          seen.add(key);
          rows.push({ domain: "", username: m[1], secret: m[4], kind: "hash" });
        }
      }
    }
  }

  // Pattern 3: simple domain\user:password or user:password
  if (rows.length === 0) {
    for (const line of raw.split(/\r?\n/)) {
      if (rows.length >= cap) break;
      const lineT = line.trim();
      if (!lineT || lineT.startsWith("#") || lineT.startsWith("//")) continue;
      const m = /^(?:([^\s:\\]+)\\)?([^\s:]+):(.+)$/.exec(lineT);
      if (!m || m.length <= 3) continue;
      const domain = (m[1] || "").trim();
      const username = (m[2] || "").trim();
      const secret = (m[3] || "").trim();
      if (!username || !secret || secret.includes("/") || secret.includes("\\") || secret.length > 256) continue;
      // Skip lines that look like URLs / paths rather than user:pass
      if (/^https?:/i.test(lineT)) continue;
      const key = `${domain}|${username}|${secret}`;
      if (!seen.has(key)) {
        seen.add(key);
        rows.push({ domain, username, secret, kind: "password" });
      }
    }
  }

  return rows;
}

function maskSecret(s: string): string {
  if (s.length <= 4) return "•".repeat(Math.max(s.length, 2));
  return `${s.slice(0, 2)}${"•".repeat(Math.min(s.length - 4, 12))}${s.slice(-2)}`;
}

export function CredsResultView({ result }: TaskRendererProps) {
  const rows = parseCredRows(result || "");
  if (rows.length === 0) return <RawFallback result={result} />;
  const hashes = rows.filter((r) => r.kind === "hash").length;
  return (
    <div className="space-y-1" data-testid="creds-renderer">
      <div className="flex items-center gap-2 mb-2 text-(--fs-micro-sm) text-muted-foreground">
        <Badge variant="secondary" className="px-1.5 py-px text-(--fs-micro) leading-none rounded bg-warning/15 text-warning font-mono">
          {rows.length}
        </Badge>
        <span>credentials</span>
        {hashes > 0 && (
          <Badge variant="secondary" className="px-1.5 py-px text-(--fs-micro) leading-none rounded bg-primary/15 text-primary font-mono">
            {hashes} hash
          </Badge>
        )}
      </div>
      <div className="overflow-x-auto scrollbar-thin max-h-72 overflow-y-auto rounded-lg border border-border">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-border text-left text-muted-foreground bg-muted/40">
              <th className="py-1 px-2 font-medium">Domain</th>
              <th className="py-1 px-2 font-medium">Username</th>
              <th className="py-1 px-2 font-medium">Secret</th>
              <th className="py-1 px-2 font-medium w-20">Type</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, i) => (
              <tr key={`${row.domain}|${row.username}|${i}`} className="border-b border-border/40 last:border-0">
                <td className="py-1 px-2 font-mono align-top">{row.domain || "-"}</td>
                <td className="py-1 px-2 font-mono align-top">{row.username}</td>
                <td className="py-1 px-2 font-mono align-top truncate max-w-[220px]" title={maskSecret(row.secret)}>
                  {maskSecret(row.secret)}
                </td>
                <td className="py-1 px-2 align-top">
                  <Badge variant="outline" className={cn("text-(--fs-micro-sm)", row.kind === "hash" ? "bg-primary/10 text-primary" : "bg-warning/10 text-warning")}>
                    {row.kind}
                  </Badge>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

registerTaskRenderer("creds", CredsResultView);
registerTaskRenderer("creds_dump", CredsResultView);
registerTaskRenderer("mimikatz", CredsResultView);
