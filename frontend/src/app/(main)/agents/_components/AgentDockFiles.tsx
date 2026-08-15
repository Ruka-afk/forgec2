"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import Link from "next/link";
import { CloudUpload, Download, File, Folder, FolderUp } from "lucide-react";
import { formatSize, joinPath, parentPath, type FileEntry } from "../[id]/files/_components/types";
import { extractImmediateListing, filesLsTaskId, isFilesLsAck, parseLsListing } from "../[id]/files/_components/ls-listing";
import { pullRemoteFile, pushLocalFile } from "../[id]/files/_components/file-transfer";
import { transferPercent, type TransferProgress } from "../[id]/files/_components/file-task";

interface AgentDockFilesProps {
  agentId: string;
  osType: "windows" | "linux";
}

export function AgentDockFiles({ agentId, osType }: AgentDockFilesProps) {
  const { t } = useI18n();
  const initial = osType === "linux" ? "/" : "C:\\";
  const [path, setPath] = useState(initial);
  const [pathInput, setPathInput] = useState(initial);
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busyName, setBusyName] = useState<string | null>(null);
  const [pushPct, setPushPct] = useState(0);
  const [pullProgress, setPullProgress] = useState<TransferProgress | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async (next: string) => {
    if (!agentId) return;
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    setLoading(true);
    setError(null);
    setEntries([]);
    try {
      const data = await api.post(paths.agents.filesLs(agentId), { path: next }, { signal: ac.signal });
      if (ac.signal.aborted) return;
      let items: FileEntry[] = extractImmediateListing(data) ?? [];
      if (isFilesLsAck(data)) {
        const taskId = filesLsTaskId(data);
        if (!taskId) throw new Error(t("agents.dock_files_waiting"));
        const st = await pollTask(agentId, taskId, { signal: ac.signal, timeoutMs: 90_000 });
        if (ac.signal.aborted) return;
        if (st.status === "failed") throw new Error(st.error || t("agents.dock_files_empty"));
        items = parseLsListing(st.result || "");
      }
      items.sort((a, b) => {
        if (a.is_dir && !b.is_dir) return -1;
        if (!a.is_dir && b.is_dir) return 1;
        return a.name.localeCompare(b.name);
      });
      setEntries(items);
      setPath(next);
      setPathInput(next);
    } catch (e) {
      if (ac.signal.aborted) return;
      setEntries([]);
      setError(e instanceof Error ? e.message : t("agents.dock_files_empty"));
    } finally {
      if (!ac.signal.aborted) setLoading(false);
    }
  }, [agentId, t]);

  useEffect(() => {
    const start = osType === "linux" ? "/" : "C:\\";
    setPath(start);
    setPathInput(start);
    void load(start);
  }, [agentId, osType, load]);

  const go = (next: string) => { void load(next); };

  const pull = async (entry: FileEntry) => {
    const remote = joinPath(path, entry.name, osType);
    setBusyName(entry.name);
    setPullProgress(null);
    toast(t("agents.files_pull_queued", { filename: entry.name }));
    try {
      const kind = await pullRemoteFile({
        agentId,
        remotePath: remote,
        filename: entry.name,
        fileSize: entry.size,
        t,
        onProgress: setPullProgress,
      });
      toast.success(kind === "partial"
        ? t("agents.files_pull_partial", { filename: entry.name, size: String(entry.size) })
        : t("agents.files_pulled", { filename: entry.name }));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("agents.files_pull_failed"));
    } finally {
      setBusyName(null);
      setPullProgress(null);
    }
  };

  const push = (file: File) => {
    const dest = joinPath(path, file.name, osType);
    setBusyName(file.name);
    setPushPct(0);
    toast(t("agents.files_push_queued", { name: file.name }));
    void pushLocalFile({
      agentId,
      destPath: dest,
      file,
      t,
      onProgress: setPushPct,
    })
      .then(() => {
        toast.success(t("agents.files_push_done", { name: file.name }));
        void load(path);
      })
      .catch((e) => {
        toast.error(e instanceof Error ? e.message : t("agents.files_push_failed"));
      })
      .finally(() => {
        setBusyName(null);
        setPushPct(0);
      });
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <form
        className="flex items-center gap-1 border-b border-border px-2 py-1"
        onSubmit={(e) => { e.preventDefault(); go(pathInput || path); }}
      >
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          onClick={() => go(parentPath(path, osType))}
          aria-label={t("agents.dock_files_up")}
        >
          <FolderUp className="size-3.5" />
        </Button>
        <Input
          value={pathInput}
          onChange={(e) => setPathInput(e.target.value)}
          aria-label={t("agents.dock_files_path")}
          className="h-7 font-mono text-xs"
        />
        <input
          ref={fileRef}
          type="file"
          className="hidden"
          aria-label={t("agents.files.select_upload")}
          onChange={(e) => {
            const file = e.target.files?.[0];
            e.target.value = "";
            if (file) push(file);
          }}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          disabled={!!busyName}
          onClick={() => fileRef.current?.click()}
          aria-label={t("agents.files_upload")}
        >
          <CloudUpload className="size-3.5" />
        </Button>
      </form>
      <p className="border-b border-border px-3 py-1 text-(--fs-micro-sm) text-muted-foreground">
        {t("agents.dock_files_transfer_hint")}{" "}
        <Link href={`/agents/${agentId}/files`} className="text-primary hover:underline">
          {t("agents.files_title")}
        </Link>
        {busyName && pushPct > 0 && (
          <span className="ml-2 font-mono">{t("agents.files_push_progress", { pct: pushPct })}</span>
        )}
        {busyName && pullProgress && (
          <span className="ml-2 font-mono">
            {t("agents.files_pull_progress", {
              current: pullProgress.chunkIndex,
              total: pullProgress.chunkCount,
              pct: transferPercent(pullProgress),
            })}
          </span>
        )}
      </p>
      <div className="min-h-0 flex-1 overflow-auto">
        {loading ? (
          <div className="flex h-full flex-col items-center justify-center gap-2 px-3">
            <Spinner size="sm" />
            <p className="text-center text-xs text-muted-foreground">{t("agents.dock_files_waiting")}</p>
          </div>
        ) : error ? (
          <p className="px-3 py-6 text-center text-xs text-destructive">{error}</p>
        ) : entries.length === 0 ? (
          <p className="px-3 py-6 text-center text-xs text-muted-foreground">{t("agents.dock_files_empty")}</p>
        ) : (
          <ul>
            {entries.map((entry) => (
              <li key={entry.name} className="flex items-center gap-1 pr-2">
                <button
                  type="button"
                  onClick={() => { if (entry.is_dir) go(joinPath(path, entry.name, osType)); }}
                  className="flex min-w-0 flex-1 items-center gap-2 px-3 py-1 text-left text-xs hover:bg-secondary/60"
                >
                  {entry.is_dir ? <Folder className="size-3.5 text-primary" /> : <File className="size-3.5 text-muted-foreground" />}
                  <span className="min-w-0 flex-1 truncate font-mono">{entry.name}</span>
                  <span className="shrink-0 font-mono text-muted-foreground">{entry.is_dir ? "" : formatSize(entry.size)}</span>
                </button>
                {!entry.is_dir && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    disabled={busyName === entry.name}
                    onClick={() => void pull(entry)}
                    aria-label={t("agents.files_pull")}
                  >
                    {busyName === entry.name ? <Spinner size="sm" /> : <Download className="size-3.5" />}
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
