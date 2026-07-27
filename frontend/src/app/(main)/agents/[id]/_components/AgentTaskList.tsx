"use client";

import { Card } from "@/components/ui/card";
import Link from "next/link";
import { StatusBadge } from "@/components/UI";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { timeAgo } from "@/lib/utils";
import type { AgentTaskRecord } from "@/types/agent";
import { Camera, ChevronDown, Clipboard, Clock, Database, Download, Folder, Globe, Image, Keyboard, ListChecks, MessageSquare, Music, Shield, Skull, StickyNote, Terminal, Upload } from "lucide-react";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";
import { useI18n } from "@/lib/i18n";

const MAX_VISIBLE_TASKS = 8;

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
    // eslint-disable-next-line jsx-a11y/alt-text -- lucide-react Image icon, not an HTML img
    case "wallpaper_change": return <Image className={s} aria-hidden="true" />;
    case "msgbox": return <MessageSquare className={s} />;
    case "play_sound": return <Music className={s} />;
    case "open_url": return <Globe className={s} />;
    case "screen_rotate": case "cursor_flip": return <Terminal className={s} />;
    case "cdrom_tray": return <Terminal className={s} />;
    case "notepad_spam": return <StickyNote className={s} />;
    case "lock_workstation": case "set_volume": return <Terminal className={s} />;
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
    case "wallpaper_change": case "msgbox": case "play_sound": case "open_url": case "screen_rotate": case "cdrom_tray": case "notepad_spam": case "lock_workstation": case "set_volume": case "cursor_flip":
      return "bg-pink-500/15 text-pink-600 dark:text-pink-400";
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
    <Card className="mb-4 overflow-hidden border-border/70 bg-card/90 shadow-sm">
      <div className="h-1 w-full bg-gradient-to-r from-primary via-cyan-500 to-emerald-500" />
      <div className="px-4 py-3 flex items-center justify-between border-b border-border/70">
        <h3 className="text-sm font-semibold text-foreground flex items-center gap-2"><ListChecks className="w-3.5 h-3.5 text-primary" />{t("agents.tasklist_recent")}</h3>
        <Link href={`/tasks?agent_id=${agentId}`} className="text-xs text-indigo-600 dark:text-indigo-400 hover:underline">{t("agents.tasklist_view_all")} &rarr;</Link>
      </div>
      <div className="divide-y divide-border/70">
        {tasks.slice(0, MAX_VISIBLE_TASKS).map((task, i) => {
          const taskId = task.id ?? i;
          const isExpanded = expandedTaskId === taskId;
          const tType = task.type || "";
          const command = (task.command || "").substring(0, 60);

          return (
            <Collapsible key={taskId} open={isExpanded} onOpenChange={(open) => { if (open) onToggleExpand(taskId); else if (expandedTaskId === taskId) onToggleExpand(taskId); }}>
              <div className="group">
                <CollapsibleTrigger className="w-full">
                  <div
                    className="px-4 py-3 flex items-start justify-between gap-3 cursor-pointer transition-colors hover:bg-muted/50"
                  >
                    <div className="flex items-start gap-3 min-w-0 flex-1">
                      <span className={`w-7 h-7 rounded-lg flex items-center justify-center shrink-0 border border-border/60 shadow-sm ${getTaskTypeColor(tType)}`}>
                        {getTaskTypeIcon(tType)}
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="text-xs font-semibold text-foreground truncate">{tType}</span>
                          {(task.command) && (
                            <span className="text-(--font-size-micro-sm) text-muted-foreground/70 font-mono truncate max-w-[220px]">
                              {command}
                            </span>
                          )}
                        </div>
                        <div className="mt-1 flex items-center gap-2 text-(--font-size-micro-sm) text-muted-foreground/70">
                          <span className="rounded-full border border-border/70 bg-muted/40 px-2 py-0.5 font-mono">#{taskId}</span>
                          {task.created_by && <span className="truncate">{task.created_by}</span>}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <StatusBadge status={task.status || "pending"} />
                      <Tooltip>
                        <TooltipTrigger>
                          <span className="rounded-full border border-border/70 bg-muted/30 px-2 py-1 text-(--font-size-micro-sm) text-muted-foreground/70 whitespace-nowrap">
                            {(task.created_at) ? timeAgo(String(task.created_at)) : ""}
                          </span>
                        </TooltipTrigger>
                        <TooltipContent>{String(task.created_at || "")}</TooltipContent>
                      </Tooltip>
                      <ChevronDown className="w-2.5 h-2.5 text-muted-foreground/70" />
                    </div>
                  </div>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className="border-t border-border/70 bg-muted/20 px-4 pb-4">
                    <div className="grid gap-2 pt-3 text-xs md:grid-cols-2">
                      <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_id")}</span> <span className="font-mono text-foreground">{taskId}</span></div>
                      <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_type")}</span> <span className="text-foreground">{tType}</span></div>
                      <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_created")}</span> <span className="text-foreground">{(task.created_at) ? new Date(String(task.created_at)).toLocaleString() : "\u2014"}</span></div>
                      {(task.created_by) && <div><span className="text-muted-foreground/70">{t("agents.tasklist_label_by")}</span> <span className="text-foreground">{task.created_by}</span></div>}
                      {(task.command) && <div className="md:col-span-2"><span className="text-muted-foreground/70">{t("agents.tasklist_label_command")}</span> <span className="font-mono text-foreground break-all">{task.command}</span></div>}
                      {(task.result) && <div className="md:col-span-2"><span className="text-muted-foreground/70">{t("agents.tasklist_label_result")}</span><pre className="mt-1 max-h-44 overflow-y-auto rounded-xl border border-border/70 bg-card p-3 font-mono text-xs text-foreground whitespace-pre-wrap break-all">{(task.result || "").substring(0, 2000)}</pre></div>}
                      {(task.error) && <div className="md:col-span-2"><span className="text-muted-foreground/70">{t("agents.tasklist_label_error")}</span><pre className="mt-1 max-h-32 overflow-y-auto rounded-xl border border-destructive/20 bg-destructive/10 p-3 font-mono text-xs text-destructive whitespace-pre-wrap break-all">{task.error}</pre></div>}
                    </div>
                  </div>
                </CollapsibleContent>
              </div>
            </Collapsible>
          );
        })}
      </div>
    </Card>
  );
}
