"use client";

import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Plus, MessageSquare, Pencil, Trash2 } from "lucide-react";

interface AISession {
  id: number;
  title: string;
  updated_at: string;
}

interface AISessionSidebarProps {
  sessions: AISession[];
  activeSessionId: number | null;
  onSelect: (id: number) => void;
  onDelete: (id: number) => void;
  onRename: (id: number, currentTitle: string) => void;
  onNewChat: () => void;
}

export default function AISessionSidebar({ sessions, activeSessionId, onSelect, onDelete, onRename, onNewChat }: AISessionSidebarProps) {
  const { t } = useI18n();

  return (
    <div className="flex h-full flex-col bg-muted/20 p-3">
      <div className="mb-3 flex items-center justify-between px-1">
        <div>
          <h2 className="text-sm font-semibold text-foreground">{t("ai.sessions")}</h2>
          <p className="text-(--fs-micro-sm) text-muted-foreground">{sessions.length} / 100</p>
        </div>
        <Button variant="ghost" size="icon-sm" onClick={onNewChat} aria-label={t("ai.new_chat")} title={t("ai.new_chat")}>
          <Plus className="size-4" />
        </Button>
      </div>
      <Button onClick={onNewChat} variant="outline" className="mb-3 h-10 shrink-0 justify-start bg-card text-xs shadow-xs">
        <Plus className="size-4" /> {t("ai.new_chat")}
      </Button>
      <div className="min-h-0 flex-1 space-y-1 overflow-y-auto pr-1">
        {sessions.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border/80 px-4 py-10 text-center">
            <div className="mb-3 flex size-9 items-center justify-center rounded-xl bg-card text-muted-foreground shadow-xs">
              <MessageSquare className="size-4" />
            </div>
            <p className="text-xs leading-5 text-muted-foreground">{t("ai.no_sessions")}</p>
          </div>
        ) : (
          sessions.map((s) => (
            <div
              key={s.id}
              className={`group relative flex min-h-11 items-center gap-1 rounded-xl border px-1.5 py-1 text-sm transition-colors ${
                activeSessionId === s.id
                  ? "border-primary/20 bg-primary/8 text-foreground"
                  : "border-transparent text-muted-foreground hover:border-border/75 hover:bg-card"
              }`}
            >
              {activeSessionId === s.id && <span className="absolute inset-y-2 left-0 w-0.5 rounded-full bg-primary" aria-hidden="true" />}
              <Button
                type="button"
                variant="ghost"
                onClick={() => onSelect(s.id)}
                className="h-auto min-w-0 flex-1 justify-start gap-2 px-1.5 py-1 text-left font-normal"
              >
                <MessageSquare className="size-4 shrink-0" aria-hidden="true" />
                <span className="flex-1 truncate">{s.title}</span>
              </Button>
              <Button
                variant="ghost" size="icon-xs"
                onClick={() => onRename(s.id, s.title)}
                className="shrink-0 text-muted-foreground opacity-100 transition-opacity hover:text-primary sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
                aria-label={t("ai.rename")}
              >
                <Pencil className="size-4" />
              </Button>
              <Button
                variant="ghost" size="icon-xs"
                onClick={() => onDelete(s.id)}
                className="shrink-0 text-muted-foreground opacity-100 transition-opacity hover:text-destructive sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
                aria-label={t("ai.delete_session")}
              >
                <Trash2 className="size-4" />
              </Button>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
