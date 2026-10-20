import { useEffect, useRef, useState } from "react";
import {
  AlertTriangle,
  Archive,
  File,
  FileText,
  FolderOpen,
  ImageIcon,
  KeyRound,
  Music,
  Search,
  Settings,
  Video,
} from "lucide-react";
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
  buildFileSearchForm,
  FILE_SEARCH_CATEGORIES,
  MEDIA_DEFAULT_CATEGORIES,
  parseFileSearchResult,
  type FileSearchCategory,
  type FileSearchHit,
  type FileSearchResult,
} from "@/lib/agent-files/file-search";
import { formatBytes, formatTimestamp } from "@/lib/agent-files/types";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface FilesMediaPanelProps {
  t: TKey;
  agentId: string;
  rootPath: string;
  onReveal: (hit: FileSearchHit) => void;
}

function categoryLabel(t: TKey, category: FileSearchCategory): string {
  switch (category) {
    case "image":
      return t("agents.files_media_image");
    case "video":
      return t("agents.files_media_video");
    case "audio":
      return t("agents.files_media_audio");
    case "document":
      return t("agents.files_media_document");
    case "archive":
      return t("agents.files_media_archive");
    case "key":
      return t("agents.files_media_key");
    case "config":
      return t("agents.files_media_config");
    default:
      return t("agents.files_media_other");
  }
}

function CategoryIcon({ category, className }: { category: FileSearchCategory; className?: string }) {
  switch (category) {
    case "image":
      return <ImageIcon className={className} />;
    case "video":
      return <Video className={className} />;
    case "audio":
      return <Music className={className} />;
    case "document":
      return <FileText className={className} />;
    case "archive":
      return <Archive className={className} />;
    case "key":
      return <KeyRound className={className} />;
    case "config":
      return <Settings className={className} />;
    default:
      return <File className={className} />;
  }
}

function mbToBytes(value: string): number {
  const n = Number(value.trim());
  if (!Number.isFinite(n) || n <= 0) return 0;
  return Math.round(n * 1024 * 1024);
}

function truncationText(t: TKey, reason: string): string {
  switch (reason) {
    case "max_results":
      return t("agents.files_media_trunc_max_results");
    case "max_bytes":
      return t("agents.files_media_trunc_max_bytes");
    case "timeout":
      return t("agents.files_media_trunc_timeout");
    case "cancelled":
      return t("agents.files_media_trunc_cancelled");
    default:
      return t("agents.files_media_trunc_unknown");
  }
}

/**
 * Metadata-only media/file search for the current Beacon directory. Search
 * filters are sent as a form (the file API's wire format); input/config files
 * stay in the encrypted task result and are never fetched or previewed here.
 */
export default function FilesMediaPanel({ t, agentId, rootPath, onReveal }: FilesMediaPanelProps) {
  const [pattern, setPattern] = useState("*");
  const [selected, setSelected] = useState<FileSearchCategory[]>(MEDIA_DEFAULT_CATEGORIES);
  const [minSize, setMinSize] = useState("");
  const [maxSize, setMaxSize] = useState("");
  const [modifiedAfter, setModifiedAfter] = useState("");
  const [modifiedBefore, setModifiedBefore] = useState("");
  const [includeHidden, setIncludeHidden] = useState(false);
  const [maxDepth, setMaxDepth] = useState("8");
  const [searching, setSearching] = useState(false);
  const [result, setResult] = useState<FileSearchResult | null>(null);
  const [error, setError] = useState("");
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    setSearching(false);
    setResult(null);
    setError("");
  }, [rootPath]);

  useEffect(() => () => {
    abortRef.current?.abort();
    abortRef.current = null;
  }, []);

  const allSelected = selected.length === 0;

  function toggleCategory(category: FileSearchCategory) {
    setSelected((current) => {
      if (current.includes(category)) {
        return current.filter((c) => c !== category);
      }
      return FILE_SEARCH_CATEGORIES.filter((c) => c === category || current.includes(c));
    });
  }

  async function runSearch() {
    if (searching || !rootPath.trim()) return;
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    setSearching(true);
    setError("");
    setResult(null);
    try {
      const depth = Math.min(16, Math.max(1, Math.floor(Number(maxDepth) || 8)));
      const form = buildFileSearchForm({
        root: rootPath,
        pattern,
        categories: selected,
        minSize: mbToBytes(minSize),
        maxSize: mbToBytes(maxSize),
        modifiedAfter,
        modifiedBefore,
        includeHidden,
        maxDepth: depth,
      });
      const ack = await api.post<Record<string, unknown>>(paths.agents.filesSearch(agentId), form);
      const taskId = fileTaskId(ack);
      if (!taskId) throw new Error(t("agents.files_media_bad_ack"));
      const status = await pollTask(agentId, taskId, { timeoutMs: 180_000, signal: ac.signal });
      if (status.status !== "completed") {
        throw new Error(status.error || t("agents.files_media_search_failed"));
      }
      const parsed = parseFileSearchResult(status.result);
      if (!parsed) throw new Error(t("agents.files_media_bad_result"));
      if (!ac.signal.aborted) setResult(parsed);
    } catch (err) {
      if (!ac.signal.aborted) {
        setError(errorText(t, err, () => t("agents.files_media_search_failed")));
      }
    } finally {
      if (abortRef.current === ac) {
        abortRef.current = null;
        if (!ac.signal.aborted) setSearching(false);
      }
    }
  }

  return (
    <div className="space-y-3">
      <Card className="p-3 sm:p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-sm font-medium">
              <Search className="size-4 text-primary" aria-hidden="true" />
              {t("agents.files_media_title")}
            </div>
            <p className="mt-1 max-w-2xl text-xs text-muted-foreground">{t("agents.files_media_hint")}</p>
          </div>
          <div className="min-w-0 max-w-full rounded-md border border-border/60 bg-muted/40 px-2 py-1 text-[11px] text-muted-foreground">
            <span className="font-medium">{t("agents.files_media_root")}:</span>{" "}
            <span className="break-all font-mono">{rootPath || "-"}</span>
          </div>
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-1.5">
          <Button
            type="button"
            size="sm"
            variant={allSelected ? "secondary" : "outline"}
            aria-pressed={allSelected}
            onClick={() => setSelected([])}
          >
            {t("agents.files_media_all")}
          </Button>
          {FILE_SEARCH_CATEGORIES.map((category) => {
            const active = selected.includes(category);
            return (
              <Button
                key={category}
                type="button"
                size="sm"
                variant={active ? "secondary" : "outline"}
                aria-pressed={active}
                onClick={() => toggleCategory(category)}
              >
                <CategoryIcon category={category} className="size-3.5" />
                {categoryLabel(t, category)}
              </Button>
            );
          })}
        </div>

        <div className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_media_pattern")}</span>
            <Input
              value={pattern}
              onChange={(e) => setPattern(e.target.value)}
              placeholder={t("agents.files_media_pattern_placeholder")}
              className="h-8 font-mono text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_media_min_size")}</span>
            <Input
              type="number"
              min="0"
              step="0.1"
              value={minSize}
              onChange={(e) => setMinSize(e.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_media_max_size")}</span>
            <Input
              type="number"
              min="0"
              step="0.1"
              value={maxSize}
              onChange={(e) => setMaxSize(e.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_media_max_depth")}</span>
            <Input
              type="number"
              min="1"
              max="16"
              value={maxDepth}
              onChange={(e) => setMaxDepth(e.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_media_modified_after")}</span>
            <Input
              type="date"
              value={modifiedAfter}
              onChange={(e) => setModifiedAfter(e.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <label className="space-y-1 text-xs font-medium">
            <span>{t("agents.files_media_modified_before")}</span>
            <Input
              type="date"
              value={modifiedBefore}
              onChange={(e) => setModifiedBefore(e.target.value)}
              className="h-8 text-xs"
            />
          </label>
          <div className="flex items-end">
            <label className="flex h-8 items-center gap-2 text-xs font-medium">
              <Checkbox
                checked={includeHidden}
                onCheckedChange={(value) => setIncludeHidden(value === true)}
              />
              {t("agents.files_media_include_hidden")}
            </label>
          </div>
          <div className="flex items-end">
            <Button
              type="button"
              size="sm"
              className="h-8 w-full"
              onClick={runSearch}
              disabled={searching || !rootPath.trim()}
            >
              {searching ? <Spinner size="xs" color="white" /> : <Search className="size-3.5" />}
              {searching ? t("agents.files_media_searching") : t("agents.files_media_search")}
            </Button>
          </div>
        </div>
      </Card>

      {error ? <DataError message={error} /> : null}

      {searching ? (
        <Card className="flex min-h-32 items-center justify-center gap-3 p-6 text-sm text-muted-foreground">
          <Spinner size="sm" />
          {t("agents.files_media_searching")}
        </Card>
      ) : null}

      {!searching && !result && !error ? (
        <Card className="border-dashed p-6 text-center text-sm text-muted-foreground">
          {t("agents.files_media_ready")}
        </Card>
      ) : null}

      {result ? (
        <Card className="overflow-hidden">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-border/60 p-3 text-xs text-muted-foreground">
            <span className="font-medium text-foreground">
              {t("agents.files_media_found", { n: result.hits.length })}
            </span>
            <span>{t("agents.files_media_scanned", { n: result.scanned })}</span>
            {result.errors > 0 ? (
              <span className="text-warning-ink">{t("agents.files_media_errors", { n: result.errors })}</span>
            ) : null}
          </div>
          {result.truncated ? (
            <div className="flex items-start gap-2 border-b border-warning/30 bg-warning/10 p-3 text-xs text-warning-ink">
              <AlertTriangle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
              <span>
                <span className="font-medium">{t("agents.files_media_truncated")}: </span>
                {truncationText(t, result.truncatedReason)}
              </span>
            </div>
          ) : null}
          {result.hits.length === 0 ? (
            <div className="p-8 text-center text-sm text-muted-foreground">
              {t("agents.files_media_empty")}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("agents.files_media_col_name")}</TableHead>
                    <TableHead>{t("agents.files_media_col_category")}</TableHead>
                    <TableHead className="text-right">{t("agents.files_media_col_size")}</TableHead>
                    <TableHead>{t("agents.files_media_col_modified")}</TableHead>
                    <TableHead className="w-10" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {result.hits.map((hit) => (
                    <TableRow
                      key={hit.path}
                      className="cursor-pointer hover:bg-muted/50"
                      onClick={() => onReveal(hit)}
                    >
                      <TableCell className="min-w-52 max-w-md">
                        <div className="flex items-start gap-2">
                          <CategoryIcon category={hit.category} className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                          <div className="min-w-0">
                            <div className="truncate font-medium" title={hit.name}>{hit.name}</div>
                            <div className="break-all font-mono text-[11px] text-muted-foreground">{hit.path}</div>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary">{categoryLabel(t, hit.category)}</Badge>
                      </TableCell>
                      <TableCell className="text-right font-mono text-xs">{formatBytes(hit.size)}</TableCell>
                      <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                        {formatTimestamp(hit.modified)}
                      </TableCell>
                      <TableCell>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          aria-label={t("agents.files_media_reveal")}
                          title={t("agents.files_media_reveal")}
                          onClick={(event) => {
                            event.stopPropagation();
                            onReveal(hit);
                          }}
                        >
                          <FolderOpen className="size-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </Card>
      ) : null}
    </div>
  );
}
