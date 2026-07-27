"use client";

import { useEffect, useState, useCallback, useRef, useMemo } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import { formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";

import { PageHeader, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ArrowLeft, Box, Camera, ChevronDown, Clipboard, Clock, Cpu, Download, FileText, Globe, History, Key, Link as LinkIcon, Link2Off, List, Network, Pause, Play, Puzzle, Power, RefreshCw, Search, Shield, Square, Syringe, Terminal, Upload, X, Zap } from "lucide-react";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";

interface RecordingEntry {
  id: number;
  task_id: number;
  agent_id: string;
  operator: string;
  action: string;
  detail: string;
  result: string;
  timestamp: string;
}

const ACTION_ICONS: Record<string, React.ReactNode> = {
  shell: <Terminal className="w-3.5 h-3.5" />,
  screenshot: <Camera className="w-3.5 h-3.5" />,
  screenshot_window: <Camera className="w-3.5 h-3.5" />,
  ps: <Cpu className="w-3.5 h-3.5" />,
  kill: <X className="w-3.5 h-3.5" />,
  inject: <Syringe className="w-3.5 h-3.5" />,
  spawn: <Zap className="w-3.5 h-3.5" />,
  elevate: <ArrowLeft className="w-3.5 h-3.5 -rotate-90" />,
  lateral: <Network className="w-3.5 h-3.5" />,
  socks: <Network className="w-3.5 h-3.5" />,
  keylogger_start: <Clipboard className="w-3.5 h-3.5" />,
  bof: <Box className="w-3.5 h-3.5" />,
  execute_assembly: <Puzzle className="w-3.5 h-3.5" />,
  mimikatz: <Shield className="w-3.5 h-3.5" />,
  powerpick: <Zap className="w-3.5 h-3.5" />,
  net: <Network className="w-3.5 h-3.5" />,
  portscan: <Search className="w-3.5 h-3.5" />,
  find: <Search className="w-3.5 h-3.5" />,
  reg_get: <FileText className="w-3.5 h-3.5" />,
  reg_set: <FileText className="w-3.5 h-3.5" />,
  reg_delete: <FileText className="w-3.5 h-3.5" />,
  download_url: <Download className="w-3.5 h-3.5" />,
  upload: <Upload className="w-3.5 h-3.5" />,
  download: <Download className="w-3.5 h-3.5" />,
  clipboard_get: <Clipboard className="w-3.5 h-3.5" />,
  clipboard_set: <Clipboard className="w-3.5 h-3.5" />,
  creds: <Key className="w-3.5 h-3.5" />,
  browser_steal: <Globe className="w-3.5 h-3.5" />,
  self_update: <RefreshCw className="w-3.5 h-3.5" />,
  beacon_now: <Zap className="w-3.5 h-3.5" />,
  reboot: <Power className="w-3.5 h-3.5" />,
  shutdown: <Power className="w-3.5 h-3.5" />,
  amsi_bypass: <Shield className="w-3.5 h-3.5" />,
  etw_bypass: <Shield className="w-3.5 h-3.5" />,
  uac_bypass: <Shield className="w-3.5 h-3.5" />,
  kerberoast: <Shield className="w-3.5 h-3.5" />,
  suspend: <Pause className="w-3.5 h-3.5" />,
  resume: <Play className="w-3.5 h-3.5" />,
  killproc: <X className="w-3.5 h-3.5" />,
  set_sleep: <Clock className="w-3.5 h-3.5" />,
  persistence_add: <LinkIcon className="w-3.5 h-3.5" />,
  persistence_list: <List className="w-3.5 h-3.5" />,
  persistence_remove: <Link2Off className="w-3.5 h-3.5" />,
  privesc_check: <Shield className="w-3.5 h-3.5" />,
};

const ACTION_COLORS: Record<string, string> = {
  shell: "text-emerald-500 bg-emerald-100 dark:bg-emerald-900/30",
  screenshot: "text-violet-500 bg-violet-100 dark:bg-violet-900/30",
  ps: "text-cyan-500 bg-cyan-100 dark:bg-cyan-900/30",
  kill: "text-red-500 bg-red-100 dark:bg-red-900/30",
  elevate: "text-orange-500 bg-orange-100 dark:bg-orange-900/30",
  creds: "text-amber-500 bg-amber-100 dark:bg-amber-900/30",
  bof: "text-indigo-500 bg-indigo-100 dark:bg-indigo-900/30",
};

const ACTION_TYPES = [
  "shell", "screenshot", "ps", "kill", "elevate",
  "creds", "bof", "inject", "lateral", "portscan",
  "keylogger_start", "mimikatz", "execute_assembly", "powerpick",
  "net", "find", "reg_get", "upload", "download",
  "clipboard_get", "browser_steal", "self_update",
  "uac_bypass", "kerberoast", "amsi_bypass", "etw_bypass",
  "suspend", "resume", "killproc", "set_sleep", "persistence_add",
  "persistence_list", "persistence_remove", "privesc_check",
  "beacon_now", "reboot", "shutdown", "spawn",
];

function getActionIcon(action: string): React.ReactNode {
  return ACTION_ICONS[action] || <Zap className="w-3.5 h-3.5" />;
}

function getActionColor(action: string): string {
  return ACTION_COLORS[action] || "text-muted-foreground bg-secondary";
}

export default function AgentRecordingPage() {
  const { t } = useI18n();
  const params = useParams();
  const id = params?.id as string;

  const [recordings, setRecordings] = useState<RecordingEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionFilter, setActionFilter] = useState("");
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [replaying, setReplaying] = useState(false);

  const listRef = useRef<HTMLDivElement>(null);
  const replayIndexRef = useRef(0);
  const replayTimerRef = useRef<NodeJS.Timeout[]>([]);

  const loadRecordings = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (actionFilter) params.set("action", actionFilter);
      const qs = params.toString();
      const data = await api.get(`/agents/${id}/recording${qs ? "?" + qs : ""}`);
      setRecordings((data.recordings as RecordingEntry[]) || []);
    } catch {
      toast.error(t("agents.recording_load_failed"));
    } finally {
      setLoading(false);
    }
  }, [id, actionFilter, t]);

  useEffect(() => { loadRecordings(); }, [loadRecordings]);

  useEffect(() => {
    return () => {
      replayTimerRef.current.forEach(clearTimeout);
      replayTimerRef.current = [];
    };
  }, []);

  const startReplay = async () => {
    try {
      const data = await api.get(`/agents/${id}/recording/replay`);
      const entries: RecordingEntry[] = (data.recordings as RecordingEntry[]) || [];
      if (entries.length === 0) return;

      setRecordings(entries);
      setReplaying(true);
      replayIndexRef.current = 0;
      replayTimerRef.current.forEach(clearTimeout);
      replayTimerRef.current = [];

      const scrollToEntry = (idx: number) => {
        if (idx >= entries.length) {
          setReplaying(false);
          return;
        }
        const entry = entries[idx];
        setExpandedId(entry.id);
        const t1 = setTimeout(() => {
          const el = document.getElementById(`recording-${entry.id}`);
          if (el) el.scrollIntoView({ behavior: "smooth", block: "center" });
          replayIndexRef.current = idx + 1;
          const t2 = setTimeout(() => scrollToEntry(idx + 1), 1500);
          replayTimerRef.current.push(t2);
        }, 500);
        replayTimerRef.current.push(t1);
      };
      scrollToEntry(0);
    } catch {
      toast.error(t("agents.recording_load_failed"));
    }
  };

  const stopReplay = () => {
    setReplaying(false);
    setExpandedId(null);
    replayTimerRef.current.forEach(clearTimeout);
    replayTimerRef.current = [];
  };

  const actionCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    recordings.forEach((r) => {
      counts[r.action] = (counts[r.action] || 0) + 1;
    });
    return counts;
  }, [recordings]);

  const countByAction = useCallback((action: string) => actionCounts[action] || 0, [actionCounts]);

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <div className="flex items-center gap-3 mb-6">
        <Link href={`/agents/${id}`} className="text-sm text-muted-foreground hover:text-foreground transition-colors">
          <ArrowLeft className="w-4 h-4" /> Agent
        </Link>
      </div>

      <PageHeader title={t("agents.recording_title")} subtitle={t("agents.recording_subtitle")}>
        <div className="flex items-center gap-2">
          {replaying ? (
            <Button onClick={stopReplay} className="h-10 rounded-xl bg-destructive hover:bg-destructive/90 text-destructive-foreground flex items-center gap-2 shrink-0">
              <Square className="w-4 h-4" />
              <span>{t("agents.recording_stop_replay")}</span>
            </Button>
          ) : (
            <Button onClick={startReplay} className="h-10 rounded-xl flex items-center gap-2 shrink-0">
              <Play className="w-4 h-4" />
              <span>{t("agents.recording_replay")}</span>
            </Button>
          )}
        </div>
      </PageHeader>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-5 mb-4">
        <Card className="p-4 text-center gap-0">
          <div className="text-2xl font-bold text-foreground">{recordings.length}</div>
          <div className="text-xs text-muted-foreground mt-1">{t("agents.recording_total_actions")}</div>
        </Card>
        <Card className="p-4 text-center gap-0">
          <div className="text-2xl font-bold text-emerald-500 dark:text-emerald-400">{countByAction("shell")}</div>
          <div className="text-xs text-muted-foreground mt-1">{t("agents.recording_shell_commands")}</div>
        </Card>
        <Card className="p-4 text-center gap-0">
          <div className="text-2xl font-bold text-violet-500 dark:text-violet-400">{countByAction("screenshot")}</div>
          <div className="text-xs text-muted-foreground mt-1">{t("agents.recording_screenshots")}</div>
        </Card>
        <Card className="p-4 text-center gap-0">
          <div className="text-2xl font-bold text-amber-500 dark:text-amber-400">{countByAction("creds") + countByAction("mimikatz") + countByAction("kerberoast")}</div>
          <div className="text-xs text-muted-foreground mt-1">{t("agents.recording_credentials")}</div>
        </Card>
      </div>

      <Card className="p-4 mb-4">
        <div className="flex flex-wrap items-center gap-3">
          <Select value={actionFilter} onValueChange={(v) => setActionFilter(v ?? "")}>
            <SelectTrigger className="min-w-[160px]">
              <SelectValue placeholder={t("agents.recording_filter_all")} />
            </SelectTrigger>
            <SelectContent>
              {ACTION_TYPES.map((a) => (
                <SelectItem key={a} value={a}>{a}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" onClick={() => { setActionFilter(""); }} className="h-10 rounded-xl">
            {t("agents.recording_reset")}
          </Button>
        </div>
      </Card>

      <div className="space-y-2" ref={listRef}>
        {loading ? (
          <div className="text-center py-16 text-muted-foreground/70">
            <Spinner />
          </div>
        ) : recordings.length === 0 ? (
          <div className="text-center py-16">
            <History className="w-4 h-4" />
            <p className="text-sm text-muted-foreground">{t("agents.recording_no_recordings")}</p>
          </div>
        ) : (
          recordings.map((entry) => {
            const icon = getActionIcon(entry.action);
            const color = getActionColor(entry.action);
            const isExpanded = expandedId === entry.id;
            return (
              <Card
                key={entry.id}
                id={`recording-${entry.id}`}
                className={`transition-all duration-300 overflow-visible py-0 shadow-sm ${isExpanded ? "ring-2 ring-indigo-500/40 shadow-md" : "hover:bg-muted"}`}
              >
                <Collapsible open={isExpanded} onOpenChange={(open) => setExpandedId(open ? entry.id : null)}>
                <CollapsibleTrigger render={<Button variant="ghost" className="w-full justify-start p-4 h-auto" />}>
                  <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${color}`}>
                    {icon}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-xs font-mono text-muted-foreground">{formatTime(entry.timestamp)}</span>
                      <Badge variant="secondary" className="text-(--font-size-micro-sm) uppercase tracking-wider">{entry.action}</Badge>
                      {entry.operator && entry.operator !== "system" && (
                        <span className="text-(--font-size-micro-sm) font-medium text-indigo-500 dark:text-indigo-400">{entry.operator}</span>
                      )}
                    </div>
                    <div className="text-sm font-medium text-foreground mt-1 truncate">{entry.detail || ""}</div>
                  </div>
                  <div className="shrink-0 text-muted-foreground/70">
                    <ChevronDown className="w-4 h-4" />
                  </div>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className="px-4 pb-4 pt-0 border-t border-border mt-0">
                    {entry.result ? (
                      <div className="mt-3">
                        <div className="text-xs font-medium text-muted-foreground mb-1">{t("agents.recording_result")}</div>
                        <pre className="text-xs font-mono bg-muted rounded-lg p-3 overflow-x-auto max-h-48 overflow-y-auto text-foreground whitespace-pre-wrap break-all">
                          {entry.result}
                        </pre>
                      </div>
                    ) : (
                      <div className="mt-3 text-xs text-muted-foreground/70 italic">{t("agents.recording_no_result")}</div>
                    )}
                    <div className="mt-2 text-(--font-size-micro-sm) text-muted-foreground/70 font-mono">
                      {t("agents.recording_recording_fmt", { id: entry.id })} &middot; {t("agents.recording_task_fmt", { id: entry.task_id })}
                    </div>
                  </div>
                </CollapsibleContent>
                </Collapsible>
              </Card>
            );
          })
        )}
      </div>
    </div>
  );
}
