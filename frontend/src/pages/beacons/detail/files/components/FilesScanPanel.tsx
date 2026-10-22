import { useEffect, useRef, useState } from "react";
import { AlertTriangle, Check, Copy, FolderOpen, ShieldAlert } from "lucide-react";
import { api, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { DataError } from "@/components/ui/data-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { errorText } from "@/lib/errors";
import { fileTaskId } from "@/lib/agent-files/file-task";
import {
  buildFileScanForm,
  FILE_SCAN_DEFAULT_RULES,
  FILE_SCAN_RULES,
  parseFileScanResult,
  type FileScanResult,
  type FileScanRule,
} from "@/lib/agent-files/file-scan";
import { formatBytes } from "@/lib/agent-files/types";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface FilesScanPanelProps {
  t: TKey;
  agentId: string;
  rootPath: string;
  onReveal: (path: string, name: string) => void;
}

function ruleLabel(t: TKey, rule: string): string {
  switch (rule) {
    case "email":
      return t("agents.files_scan_rule_email");
    case "phone":
      return t("agents.files_scan_rule_phone");
    case "bank_card":
      return t("agents.files_scan_rule_bank_card");
    case "ssn":
      return t("agents.files_scan_rule_ssn");
    case "aws_key":
      return t("agents.files_scan_rule_aws_key");
    case "private_key":
      return t("agents.files_scan_rule_private_key");
    case "jwt":
      return t("agents.files_scan_rule_jwt");
    case "password":
      return t("agents.files_scan_rule_password");
    default:
      return rule;
  }
}

function truncationText(t: TKey, reason: string): string {
  switch (reason) {
    case "max_findings":
      return t("agents.files_scan_trunc_max_findings");
    case "max_files":
      return t("agents.files_scan_trunc_max_files");
    case "max_total_bytes":
      return t("agents.files_scan_trunc_max_total_bytes");
    case "max_bytes":
      return t("agents.files_scan_trunc_max_bytes");
    case "timeout":
      return t("agents.files_scan_trunc_timeout");
    case "cancelled":
      return t("agents.files_scan_trunc_cancelled");
    default:
      return t("agents.files_scan_trunc_unknown");
  }
}

function boundedInt(value: string, fallback: number, min: number, max: number): number {
  const n = Number(value.trim());
  if (!Number.isFinite(n) || n <= 0) return fallback;
  return Math.min(max, Math.max(min, Math.floor(n)));
}

function mbToBytes(value: string): number {
  const n = Number(value.trim());
  if (!Number.isFinite(n) || n <= 0) return 0;
  return Math.round(n * 1024 * 1024);
}

/**
 * Explicit sensitive-data scan. Values are rendered raw because this is the
 * operator-authorized surface; the server keeps them in the encrypted task
 * result and redacts them from AI, callbacks, IOC extraction and exports.
 */
export default function FilesScanPanel({ t, agentId, rootPath, onReveal }: FilesScanPanelProps) {
  const [pattern, setPattern] = useState("*");
  const [rules, setRules] = useState<FileScanRule[]>([...FILE_SCAN_DEFAULT_RULES]);
  const [includeHidden, setIncludeHidden] = useState(false);
  const [maxDepth, setMaxDepth] = useState("8");
  const [maxFiles, setMaxFiles] = useState("200");
  const [maxFileMB, setMaxFileMB] = useState("1");
  const [maxTotalMB, setMaxTotalMB] = useState("32");
  const [maxFindings, setMaxFindings] = useState("500");
  const [timeoutSeconds, setTimeoutSeconds] = useState("30");
  const [scanning, setScanning] = useState(false);
  const [result, setResult] = useState<FileScanResult | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState("");
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    setScanning(false);
    setResult(null);
    setError("");
    setCopied("");
  }, [rootPath]);

  useEffect(() => () => abortRef.current?.abort(), []);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(""), 1500);
    return () => window.clearTimeout(timer);
  }, [copied]);

  function toggleRule(rule: FileScanRule) {
    setRules((current) =>
      current.includes(rule) ? current.filter((item) => item !== rule) : FILE_SCAN_RULES.filter((item) => item === rule || current.includes(item)),
    );
  }

  async function runScan() {
    if (scanning || !rootPath.trim() || rules.length === 0) return;
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    setScanning(true);
    setError("");
    setResult(null);
    setCopied("");
    try {
      const form = buildFileScanForm({
        root: rootPath,
        pattern,
        rules,
        includeHidden,
        maxDepth: boundedInt(maxDepth, 8, 1, 16),
        maxFiles: boundedInt(maxFiles, 200, 1, 1000),
        maxFileBytes: mbToBytes(maxFileMB),
        maxTotalBytes: mbToBytes(maxTotalMB),
        maxFindings: boundedInt(maxFindings, 500, 1, 2000),
        timeoutSeconds: boundedInt(timeoutSeconds, 30, 1, 60),
      });
      const ack = await api.post<Record<string, unknown>>(paths.agents.filesScan(agentId), form);
      const taskId = fileTaskId(ack);
      if (!taskId) throw new Error(t("agents.files_scan_bad_ack"));
      const status = await pollTask(agentId, taskId, { timeoutMs: 120_000, signal: ac.signal });
      if (status.status !== "completed") {
        throw new Error(status.error || t("agents.files_scan_failed"));
      }
      const parsed = parseFileScanResult(status.result || "");
      if (!parsed) throw new Error(t("agents.files_scan_bad_result"));
      if (!ac.signal.aborted) setResult(parsed);
    } catch (err) {
      if (!ac.signal.aborted) {
        setError(errorText(t, err, () => t("agents.files_scan_failed")));
      }
    } finally {
      if (abortRef.current === ac) {
        abortRef.current = null;
        if (!ac.signal.aborted) setScanning(false);
      }
    }
  }

  async function copyFinding(key: string, value: string) {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(key);
    } catch {
      setError(t("agents.files_scan_copy_failed"));
    }
  }

  return (
    <div className="space-y-3">
      <Card className="p-(--card-spacing)">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-sm font-medium">
              <ShieldAlert className="size-4 text-primary" aria-hidden="true" />
              {t("agents.files_scan_title")}
            </div>
            <p className="mt-1 max-w-2xl text-xs text-muted-foreground">{t("agents.files_scan_hint")}</p>
          </div>
          <div className="min-w-0 max-w-full rounded-md border border-border/60 bg-muted/40 px-2 py-1 text-[11px] text-muted-foreground">
            <span className="font-medium">{t("agents.files_scan_root")}:</span>{" "}
            <span className="break-all font-mono">{rootPath || "-"}</span>
          </div>
        </div>

        <div className="mt-3 flex items-start gap-2 rounded-md border border-warning/30 bg-warning/10 p-2.5 text-xs text-warning-ink">
          <AlertTriangle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          <span>{t("agents.files_scan_raw_warning")}</span>
        </div>

        <div className="mt-3 space-y-2">
          <div className="text-xs font-medium">{t("agents.files_scan_rules")}</div>
          <div className="flex flex-wrap gap-x-4 gap-y-2">
            {FILE_SCAN_RULES.map((rule) => {
              const active = rules.includes(rule);
              return (
                <label key={rule} className="flex items-center gap-2 text-xs">
                  <Checkbox checked={active} onCheckedChange={() => toggleRule(rule)} />
                  {ruleLabel(t, rule)}
                  {FILE_SCAN_DEFAULT_RULES.includes(rule) ? (
                    <Badge variant="outline" className="px-1 py-0 text-[10px]">
                      {t("agents.files_scan_default_badge")}
                    </Badge>
                  ) : null}
                </label>
              );
            })}
          </div>
          <p className="text-[11px] text-muted-foreground">{t("agents.files_scan_default_rules_hint")}</p>
          {rules.length === 0 ? (
            <p className="text-xs text-warning-ink" role="alert">
              {t("agents.files_scan_pick_rule")}
            </p>
          ) : null}
        </div>

        <div className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          <label className="space-y-1 text-xs font-medium sm:col-span-2">
            <span>{t("agents.files_scan_pattern")}</span>
            <Input
              value={pattern}
              onChange={(event) => setPattern(event.target.value)}
              placeholder="*"
              className="h-8 font-mono text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_scan_max_depth")}</span>
            <Input
              type="number"
              min="1"
              max="16"
              value={maxDepth}
              onChange={(event) => setMaxDepth(event.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_scan_max_files")}</span>
            <Input
              type="number"
              min="1"
              max="1000"
              value={maxFiles}
              onChange={(event) => setMaxFiles(event.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_scan_max_file_mb")}</span>
            <Input
              type="number"
              min="1"
              max="8"
              value={maxFileMB}
              onChange={(event) => setMaxFileMB(event.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_scan_max_total_mb")}</span>
            <Input
              type="number"
              min="1"
              max="64"
              value={maxTotalMB}
              onChange={(event) => setMaxTotalMB(event.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_scan_max_findings")}</span>
            <Input
              type="number"
              min="1"
              max="2000"
              value={maxFindings}
              onChange={(event) => setMaxFindings(event.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_scan_timeout")}</span>
            <Input
              type="number"
              min="1"
              max="60"
              value={timeoutSeconds}
              onChange={(event) => setTimeoutSeconds(event.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <div className="flex items-end">
            <label className="flex h-8 items-center gap-2 text-xs font-medium">
              <Checkbox
                checked={includeHidden}
                onCheckedChange={(value) => setIncludeHidden(value === true)}
              />
              {t("agents.files_scan_include_hidden")}
            </label>
          </div>
          <div className="flex items-end sm:col-span-2 lg:col-span-1">
            <Button
              type="button"
              size="sm"
              className="h-8 w-full"
              onClick={() => void runScan()}
              disabled={scanning || !rootPath.trim() || rules.length === 0}
            >
              {scanning ? <Spinner size="xs" color="white" /> : <ShieldAlert className="size-3.5" />}
              {scanning ? t("agents.files_scan_running") : t("agents.files_scan_start")}
            </Button>
          </div>
        </div>
      </Card>

      {error ? <DataError message={error} /> : null}

      {scanning ? (
        <Card className="flex min-h-32 items-center justify-center gap-3 p-(--card-spacing) text-sm text-muted-foreground">
          <Spinner size="sm" />
          {t("agents.files_scan_running")}
        </Card>
      ) : null}

      {!scanning && !result && !error ? (
        <Card className="border-dashed p-(--card-spacing) text-center text-sm text-muted-foreground">
          {t("agents.files_scan_ready")}
        </Card>
      ) : null}

      {result ? (
        <Card className="overflow-hidden">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-border/60 p-3 text-xs text-muted-foreground">
            <span className="font-medium text-foreground">
              {t("agents.files_scan_found", { n: result.findings.length })}
            </span>
            <span>{t("agents.files_scan_summary", { files: result.filesScanned, bytes: formatBytes(result.bytesScanned) })}</span>
            {result.filesSkipped > 0 ? (
              <span>{t("agents.files_scan_skipped", { binary: result.skippedBinary, large: result.skippedLarge })}</span>
            ) : null}
            {result.errors > 0 ? (
              <span className="text-warning-ink">{t("agents.files_scan_errors", { n: result.errors })}</span>
            ) : null}
          </div>

          {result.truncated ? (
            <div className="flex items-start gap-2 border-b border-warning/30 bg-warning/10 p-3 text-xs text-warning-ink">
              <AlertTriangle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
              <span>
                <span className="font-medium">{t("agents.files_scan_truncated")}: </span>
                {truncationText(t, result.truncatedReason)}
              </span>
            </div>
          ) : null}

          {result.findings.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">{t("agents.files_scan_empty")}</div>
          ) : (
            <div className="overflow-x-auto">
              <Table className="min-w-[900px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("agents.files_scan_rule_col")}</TableHead>
                    <TableHead>{t("agents.files_scan_value_col")}</TableHead>
                    <TableHead>{t("agents.files_scan_file_col")}</TableHead>
                    <TableHead className="w-16 text-right">{t("agents.files_scan_line_col")}</TableHead>
                    <TableHead className="w-16 text-right">{t("agents.files_scan_count_col")}</TableHead>
                    <TableHead className="w-24 text-right">{t("agents.files_scan_actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {result.findings.map((finding) => {
                    const key = `${finding.path}:${finding.line}:${finding.rule}:${finding.value}`;
                    return (
                      <TableRow key={key}>
                        <TableCell>
                          <Badge variant="secondary">{ruleLabel(t, finding.rule)}</Badge>
                        </TableCell>
                        <TableCell>
                          <span className="block max-w-[28rem] whitespace-pre-wrap break-all font-mono text-xs">
                            {finding.value}
                          </span>
                        </TableCell>
                        <TableCell>
                          <button
                            type="button"
                            className="max-w-[18rem] truncate text-left text-xs text-primary hover:underline"
                            title={finding.path}
                            onClick={() => onReveal(finding.path, finding.name)}
                          >
                            {finding.name}
                          </button>
                          <div className="max-w-[18rem] truncate font-mono text-[10px] text-muted-foreground" title={finding.path}>
                            {finding.path}
                          </div>
                        </TableCell>
                        <TableCell className="text-right font-mono text-xs">{finding.line}</TableCell>
                        <TableCell className="text-right font-mono text-xs">{finding.occurrences}</TableCell>
                        <TableCell className="text-right">
                          <div className="flex justify-end gap-0.5">
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-sm"
                              aria-label={t("agents.files_scan_copy")}
                              title={t("agents.files_scan_copy")}
                              onClick={() => void copyFinding(key, finding.value)}
                            >
                              {copied === key ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
                            </Button>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-sm"
                              aria-label={t("agents.files_scan_reveal")}
                              title={t("agents.files_scan_reveal")}
                              onClick={() => onReveal(finding.path, finding.name)}
                            >
                              <FolderOpen className="size-3.5" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </Card>
      ) : null}
    </div>
  );
}
