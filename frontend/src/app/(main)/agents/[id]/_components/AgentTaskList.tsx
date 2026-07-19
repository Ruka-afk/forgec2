"use client";

import { Card } from "@/components/ui/card";
import Link from "next/link";
import { StatusBadge } from "@/components/UI";
import { timeAgo } from "@/lib/utils";
import type { AgentTaskRecord } from "@/types/agent";
import { Camera, ChevronDown, Clipboard, Clock, Database, Download, Folder, Keyboard, ListChecks, Shield, Skull, Terminal, Upload } from "lucide-react";
import { useI18n } from "@/lib/i18n";

function getTaskTypeIcon(type: string): React.ReactNode {
  const s = "w-2.5 h-2.5";
  switch (type) {
    case "shell": return <Terminal className={s} />;
    case "screenshot": return <Camera className={s} />;
    case "ps": return <ListChecks className={s} />;
    case "kill": return <Skull className={s} />;
    case "ls": return <Folder className={s} />;
    case "download": return <Download className={s} />;
    case "upload": return <Upload className={s} />;
    case "keylogger_start": case "keylogger_dump": case "keylogger_stop": return <Keyboard className={s} />;
    case "clipboard_get": return <Clipboard className={s} />;
    case "creds_dump": return <Database className={s} />;
    case "privesc_check": return <Shield className={s} />;
    case "sleep": return <Clock className={s} />;
    case "hashdump": return <Database className={s} />;
    default: return <Terminal className={s} />;
  }
}

function getTaskTypeColor(type: string): string {
  switch (type) {
    case "shell": return "bg-indigo-500/15 text-indigo-600 dark:text-indigo-400";
    case "screenshot": return "bg-cyan-500/15 text-cyan-600 dark:text-cyan-400";
    case "ps": return "bg-muted text-muted-foreground";
    case "kill": return "bg-destructive/10 text-destructive";
    case "hashdump": case "creds_dump": return "bg-amber-500/15 text-amber-600 dark:text-amber-400";
    case "privesc_check": return "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400";
    case "clipboard_get": return "bg-cyan-500/15 text-cyan-600 dark:text-cyan-400";
    case "keylogger_start": case "keylogger_dump": case "keylogger_stop": return "bg-purple-500/15 text-purple-600 dark:text-purple-400";
    default: return "bg-muted text-muted-foreground";
  }
}

export interface AgentTaskListProps {
  tasks: AgentTaskRecord[];
  agentId: string;
  expandedTaskId: number | null;
  onToggleExpand: (id: number) => void;
}

export default function AgentTaskList({ tasks, agentId, expandedTaskId, onToggleExpand }: AgentTaskListProps) {
  const { t } = useI18n();
  if (tasks.length === 0) return null;

  return (
    <Card className="mb-4 gap-0">
      <div className="px-4 py-3 border-b border-border flex items-center justify-between">
        <h3 className="text-sm font-semibold text-foreground"><ListChecks className="w-4 h-4" />{t("agents.tasklist_recent")}</h3>
        <Link href={`/tasks?agent_id=${agentId}`} className="text-xs text-indigo-600 dark:text-indigo-400 hover:underline">{t("agents.tasklist_view_all")} &rarr;</Link>
      </div>
      <div className="divide-y divide-border">{tasks.slice(0, 8).map((task, i) => { const taskId = task.id ?? i; const isExpanded = expandedTaskId === taskId; const tType = task.type || ""; return (
        <div key={taskId}>
          <div className="px-4 py-2.5 flex items-center justify-between gap-3 cursor-pointer hover:bg-muted transition-colors" onClick={() => onToggleExpand(taskId)}>
            <div className="flex items-center gap-2.5 min-w-0 flex-1">
              <span className={`w-6 h-6 rounded-md flex items-center justify-center shrink-0 ${getTaskTypeColor(tType)}`}>{getTaskTypeIcon(tType)}</span>
              <span className="text-xs font-medium text-foreground">{tType}</span>
              {(task.command) && <span className="text-[11px] text-muted-foreground/70 font-mono truncate max-w-[200px]">{(task.command || "").substring(0, 60)}</span>}
            </div>
            <div className="flex items-center gap-2.5 shrink-0">
              <StatusBadge status={task.status || "pending"} />
              <span className="text-[10px] text-muted-foreground/70 whitespace-nowrap" title={String(task.created_at || "")}>{(task.created_at) ? timeAgo(String(task.created_at)) : ""}</span>
              <ChevronDown className={`w-2.5 h-2.5 text-muted-foreground/70 transition-transform ${isExpanded ? "rotate-180" : ""}`} />
            </div>
          </div>
          {isExpanded && (<div className="px-4 pb-3 bg-muted/30 border-t border-border"><div className="py-2 space-y-1.5 text-xs">
            <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_id")}</span> <span className="font-mono text-foreground">{taskId}</span></div>
            <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_type")}</span> <span className="text-foreground">{tType}</span></div>
            <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_created")}</span> <span className="text-foreground">{(task.created_at) ? new Date(String(task.created_at)).toLocaleString() : "\u2014"}</span></div>
            {(task.command) && <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_command")}</span> <span className="font-mono text-foreground break-all">{task.command}</span></div>}
            {(task.created_by) && <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_by")}</span> <span className="text-foreground">{task.created_by}</span></div>}
            {(task.result) && <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_result")}</span><pre className="mt-1 p-2 bg-card border border-border rounded-lg font-mono text-[11px] text-foreground whitespace-pre-wrap break-all max-h-40 overflow-y-auto">{(task.result || "").substring(0, 2000)}</pre></div>}
            {(task.error) && <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_error")}</span><pre className="mt-1 p-2 bg-destructive/10 border-destructive/20 border rounded-lg font-mono text-[11px] text-destructive whitespace-pre-wrap break-all max-h-32 overflow-y-auto">{task.error}</pre></div>}
          </div></div>)}
        </div>); })}</div>
    </Card>
  );
}
