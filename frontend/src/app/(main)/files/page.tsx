"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { formatTime } from "@/lib/utils";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { EmptyState, PageHeader, Spinner } from "@/components/UI";
import { DataState } from "@/components/ui/data-state";
import { NormalizedAgent as Agent } from "@/types/agent";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { File, FolderOpen, MousePointerClick, Search } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";

interface FileEntry {
  name: string;
  is_dir: boolean;
  size: number;
  mod_time: string;
}

export default function FilesPage() {
  const { t } = useI18n();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);
  const [currentPath, setCurrentPath] = useState("C:\\");
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filesLoading, setFilesLoading] = useState(false);

  const loadAgents = useCallback(() => {
    setLoading(true);
    setError(null);
    api.get("/agents?page=1&pageSize=200")
      .then((data) => {
        const list = data.agents || data || [];
        setAgents(list as Agent[]);
      })
      .catch(() => {
        setAgents([]);
        setError(t("files.toast.load_agents_failed"));
        toast.error(t("files.toast.load_agents_failed"));
      })
      .finally(() => setLoading(false));
  }, [t]);

  useEffect(() => { loadAgents(); }, [loadAgents]);

  const loadFiles = useCallback((agentId: string, path: string) => {
    if (!agentId) return;
    setFilesLoading(true);
    api.post(`/agents/${agentId}/files/ls`, { path })
      .then((data) => {
        const items: FileEntry[] = (data.files || data.entries || data.data || []) as FileEntry[];
        setFiles(items);
      })
      .catch(() => {
        setFiles([]);
        toast.error(t("files.toast.load_files_failed"));
      })
      .finally(() => setFilesLoading(false));
  }, [t]);

  const selectAgent = (agent: Agent) => {
    setSelectedAgent(agent);
    setCurrentPath("C:\\");
    setFiles([]);
    setTimeout(() => loadFiles(agent.id, "C:\\"), 100);
  };

  const navigateDir = (name: string) => {
    if (!selectedAgent) return;
    const newPath = currentPath.endsWith("\\") ? currentPath + name : currentPath + "\\" + name;
    setCurrentPath(newPath);
    loadFiles(selectedAgent.id, newPath);
  };

  const navigateUp = () => {
    if (!selectedAgent || currentPath === "C:\\") return;
    const parts = currentPath.replace(/\\$/, "").split("\\");
    parts.pop();
    const parent = parts.join("\\") + "\\";
    setCurrentPath(parent);
    loadFiles(selectedAgent.id, parent);
  };

  const filteredAgents = agents;

  const formatSize = (bytes: number): string => {
    if (bytes === 0) return "-";
    const units = ["B", "KB", "MB", "GB"];
    let i = 0;
    let size = bytes;
    while (size >= 1024 && i < units.length - 1) { size /= 1024; i++; }
    return size.toFixed(1) + " " + units[i];
  };

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("files.title")} subtitle={t("files.subtitle")} />

      <DataState loading={loading} error={error} onRetry={loadAgents}>
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
        <div className="lg:col-span-1">
          <Card className="">
            <div className="divide-y divide-border max-h-[70vh] overflow-y-auto">
              {loading ? (
                <div className="p-4 sm:p-5 text-center text-muted-foreground/70 text-sm">
                  <Spinner size="sm" />{t("files.loading")}
                </div>
              ) : filteredAgents.length > 0 ? (
                filteredAgents.map((a) => (
                  <Button
                    key={a.id}
                    variant="ghost"
                    onClick={() => selectAgent(a)}
                    className={`w-full justify-start text-left px-3 py-3 hover:bg-muted transition-colors rounded-none ${
                      selectedAgent?.id === a.id ? "bg-primary/10 dark:bg-indigo-900/20 border-l-2 border-indigo-500" : ""
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <span className={`w-2 h-2 rounded-full shrink-0 ${a.status === "online" ? "bg-emerald-500" : "bg-muted-foreground"}`}></span>
                      <span className="font-medium text-sm text-foreground truncate">{a.hostname || a.id.substring(0, 8)}</span>
                    </div>
                    <div className="text-xs text-muted-foreground/70 mt-0.5 ml-4 truncate">{a.ip} · {a.os}</div>
                  </Button>
                ))
              ) : (
                <div className="p-4 sm:p-5 text-center text-muted-foreground/70 text-sm">
                  <EmptyState icon={Search} title={t("files.no_agents")} message={t("files.no_agents_hint")} />
                </div>
              )}
            </div>
          </Card>
        </div>

        <div className="lg:col-span-3">
          {selectedAgent ? (
            <Card className="">
              <div className="px-4 py-3 border-b border-border flex items-center gap-2">
                <Tooltip>
                  <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={navigateUp} aria-label="Navigate to parent directory" />}>
                  <FolderOpen className="w-4 h-4" />
                  </TooltipTrigger>
                  <TooltipContent>Up</TooltipContent>
                </Tooltip>
                <span className="text-sm font-mono text-muted-foreground">{selectedAgent.hostname || selectedAgent.id.substring(0, 8)}</span>
                <span className="text-xs text-muted-foreground/70">/</span>
                <span className="text-sm font-mono text-foreground truncate">{currentPath}</span>
              </div>

              {filesLoading ? (
                <div className="p-4 sm:p-5 text-center text-muted-foreground/70">
                  <Spinner size="sm" className="mb-2" />
                   {t("files.loading_files")}
                </div>
              ) : files.length > 0 ? (
                <div className="divide-y divide-border">
                  {files.filter((f) => f.name).map((f) => (
                    <div
                      key={f.name}
                      role={f.is_dir ? "button" : undefined}
                      tabIndex={f.is_dir ? 0 : undefined}
                      onClick={() => f.is_dir && navigateDir(f.name)}
                      onKeyDown={f.is_dir ? (e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); navigateDir(f.name); } } : undefined}
                      className={`flex items-center gap-3 px-4 py-2.5 text-sm transition-colors ${
                        f.is_dir ? "cursor-pointer hover:bg-muted" : ""
                      }`}
                    >
                      {f.is_dir ? <FolderOpen className="w-5 h-5 text-amber-400" /> : <File className="w-5 h-5 text-muted-foreground" />}
                      <span className={`flex-1 truncate ${f.is_dir ? "font-medium text-foreground" : "text-muted-foreground"}`}>
                        {f.name}
                      </span>
                      {!f.is_dir && <span className="text-xs text-muted-foreground/70 w-20 text-right">{formatSize(f.size)}</span>}
                      <span className="text-xs text-muted-foreground/70 w-32 text-right hidden sm:block">{formatTime(f.mod_time)}</span>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="p-4 sm:p-5 text-center text-muted-foreground/70">
                  <FolderOpen className="w-4 h-4 mb-2" />
                  {t("files.no_files")}
                </div>
              )}
            </Card>
          ) : (
            <Card className="p-12 text-center text-muted-foreground/70">
              <MousePointerClick className="w-4 h-4" />
              <p className="text-sm">{t("files.select_agent")}</p>
            </Card>
          )}
        </div>
      </div>
      </DataState>
    </div>
  );
}

