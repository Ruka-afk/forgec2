"use client";

import { useMemo, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Archive, LoaderCircle, MessageSquare, Pencil, Pin, Plus, Search, Trash2 } from "lucide-react";

interface AISession {
  id: number;
  title: string;
  updated_at: string;
	pinned?: boolean;
	archived?: boolean;
}

interface AISessionSidebarProps {
  sessions: AISession[];
  activeSessionId: number | null;
  onSelect: (id: number) => void;
  onDelete: (id: number) => void;
  onRename: (id: number, currentTitle: string) => void;
  onNewChat: () => void;
	onPin?: (id: number, pinned: boolean) => void;
	onArchive?: (id: number) => void;
  selectingSessionId?: number | null;
	runStatuses?: Record<number, string>;
}

function formatSessionWhen(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const now = new Date();
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  return d.toLocaleDateString();
}

export default function AISessionSidebar({ sessions, activeSessionId, onSelect, onDelete, onRename, onNewChat, onPin, onArchive, selectingSessionId, runStatuses = {} }: AISessionSidebarProps) {
  const { t } = useI18n();
  const { confirm, modal } = useConfirm();
  const [query, setQuery] = useState("");
  const handleArchive = async (id: number) => {
    const ok = await confirm({ title: t("ai.archive_session"), message: t("ai.archive_confirm") || "Archive this session? You can restore it later.", confirmText: t("ai.archive_session") });
    if (ok) onArchive?.(id);
  };
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = q ? sessions.filter((s) => s.title.toLowerCase().includes(q)) : [...sessions];
    list.sort((a, b) => {
      if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
      return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
    });
    return list;
  }, [sessions, query]);

  return (
    <div className="flex h-full flex-col bg-muted/20 p-3">
      <div className="mb-3 flex items-center justify-between px-1">
        <div>
          <h2 className="text-sm font-semibold text-foreground">{t("ai.sessions")}</h2>
		  <p className="text-(--fs-micro-sm) text-muted-foreground">{t("ai.session_count", { count: sessions.length })}</p>
        </div>
        <Button variant="ghost" size="icon-sm" onClick={onNewChat} aria-label={t("ai.new_chat")} title={t("ai.new_chat")}>
          <Plus className="size-4" />
        </Button>
      </div>
      <Button onClick={onNewChat} variant="outline" className="mb-3 h-10 shrink-0 justify-start bg-card text-xs shadow-xs">
        <Plus className="size-4" /> {t("ai.new_chat")}
      </Button>
      {sessions.length > 4 && (
        <div className="relative mb-3">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("ai.search_sessions")}
            aria-label={t("ai.search_sessions")}
            className="h-8 bg-card pl-8 pr-7 text-xs"
          />
          {query && (
            <Button variant="ghost" size="icon-xs" className="absolute right-1 top-1/2 -translate-y-1/2" onClick={() => setQuery("")} aria-label={t("ai.clear_search")}>
              <Trash2 className="size-3" />
            </Button>
          )}
        </div>
      )}
      {modal}
      <div className="min-h-0 flex-1 space-y-1 overflow-y-auto pr-1">
        {filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border/80 px-4 py-10 text-center">
            <div className="mb-3 flex size-9 items-center justify-center rounded-xl bg-card text-muted-foreground shadow-xs">
              <MessageSquare className="size-4" />
            </div>
            <p className="text-xs leading-5 text-muted-foreground">{query.trim() ? t("ai.no_session_matches") : t("ai.no_sessions")}</p>
          </div>
        ) : (
          filtered.map((s) => (
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
                disabled={selectingSessionId === s.id}
                className="h-auto min-w-0 flex-1 justify-start gap-2 px-1.5 py-1 text-left font-normal"
              >
                {selectingSessionId === s.id
                  ? <LoaderCircle className="size-4 shrink-0 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                  : <MessageSquare className="size-4 shrink-0" aria-hidden="true" />}
                <span className="flex min-w-0 flex-1 flex-col items-start">
				  <span className="flex w-full items-center gap-1.5"><span className="min-w-0 flex-1 truncate">{s.title}</span>{runStatuses[s.id] && <span className="size-1.5 shrink-0 animate-pulse rounded-full bg-primary motion-reduce:animate-none" title={runStatuses[s.id]} />}</span>
                  <span className="text-(--fs-micro-sm) font-normal text-muted-foreground">{formatSessionWhen(s.updated_at)}</span>
                </span>
              </Button>
              <Button
				variant="ghost" size="icon-xs"
				onClick={() => onPin?.(s.id, !s.pinned)}
				className={s.pinned ? "shrink-0 text-primary" : "shrink-0 text-muted-foreground opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"}
				aria-label={s.pinned ? t("ai.unpin_session") : t("ai.pin_session")}
			  >
				<Pin className="size-3.5" />
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
				onClick={() => handleArchive(s.id)}
				className="shrink-0 text-muted-foreground opacity-100 transition-opacity hover:text-warning sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
				aria-label={t("ai.archive_session")}
			  >
				<Archive className="size-3.5" />
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
