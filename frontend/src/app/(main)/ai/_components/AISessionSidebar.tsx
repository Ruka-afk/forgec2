"use client";

import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Plus, MessageSquare, Pencil, Trash2 } from "lucide-react";

export interface AISession {
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
    <div className="flex flex-col h-full">
      <Button onClick={onNewChat} className="h-9 text-xs mb-2 shrink-0">
        <Plus className="w-4 h-4" /> {t("ai.new_chat")}
      </Button>
      <div className="flex-1 min-h-0 overflow-y-auto space-y-1 pr-1">
        {sessions.length === 0 ? (
          <p className="text-xs text-muted-foreground px-2 py-3">{t("ai.no_sessions")}</p>
        ) : (
          sessions.map((s) => (
            <div
              key={s.id}
              className={`group flex items-center gap-1 rounded-lg px-1.5 py-1 cursor-pointer text-sm ${
                activeSessionId === s.id
                  ? "bg-primary/10 text-primary dark:text-primary"
                  : "hover:bg-muted transition-colors text-muted-foreground"
              }`}
            >
              <Button
                type="button"
                variant="ghost"
                onClick={() => onSelect(s.id)}
                className="h-auto flex-1 min-w-0 justify-start gap-2 px-1.5 py-1 text-left font-normal"
              >
                <MessageSquare className="w-4 h-4 shrink-0" aria-hidden="true" />
                <span className="flex-1 truncate">{s.title}</span>
              </Button>
              <Button
                variant="ghost" size="icon-xs"
                onClick={() => onRename(s.id, s.title)}
                className="opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 text-muted-foreground hover:text-primary transition-opacity shrink-0"
                aria-label={t("ai.rename")}
              >
                <Pencil className="w-4 h-4" />
              </Button>
              <Button
                variant="ghost" size="icon-xs"
                onClick={() => onDelete(s.id)}
                className="opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 text-muted-foreground hover:text-destructive transition-opacity shrink-0"
                aria-label={t("ai.delete_session")}
              >
                <Trash2 className="w-4 h-4" />
              </Button>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
