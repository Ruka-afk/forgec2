"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import type { AIMessage, AISession } from "./types";

export function useAISessions(setMessagesBoth: (updater: AIMessage[] | ((prev: AIMessage[]) => AIMessage[])) => void) {
  const { t } = useI18n();
  const [sessions, setSessions] = useState<AISession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<number | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [renameTarget, setRenameTarget] = useState<{ id: number; current: string } | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const selectSeqRef = useRef(0);

  const loadSessions = useCallback(async () => {
    try {
      const data = await api.get(paths.ai.sessions);
      const list = Array.isArray(data) ? data : ((data as { data?: AISession[] })?.data ?? []);
      setSessions(list as AISession[]);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.toast.load_sessions_failed"));
    }
  }, [t]);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  const selectSession = async (id: number) => {
    const seq = ++selectSeqRef.current;
    try {
      const data = await api.get(paths.ai.sessionMessages(id));
      if (seq !== selectSeqRef.current) return;
      const list = Array.isArray(data) ? data : ((data as { data?: unknown[] })?.data ?? []);
      const mapped = (list as { role: string; content: string; tool_name?: string }[]).map(
        (m) => ({
          role: m.role as AIMessage["role"],
          content: m.content,
          tool_name: m.tool_name,
          thinking: false,
        }),
      );
      setMessagesBoth(mapped);
      setActiveSessionId(id);
      setSidebarOpen(false);
    } catch (e) {
      if (seq !== selectSeqRef.current) return;
      toast.error(e instanceof Error ? e.message : t("ai.toast.load_session_messages_failed"));
    }
  };

  const deleteSession = async (id: number) => {
    try {
      await api.del(paths.ai.session(id));
      setSessions((prev) => prev.filter((s) => s.id !== id));
      if (activeSessionId === id) {
        setActiveSessionId(null);
        setMessagesBoth([]);
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.toast.delete_session_failed"));
    }
  };

  const handleNewChat = () => {
    setMessagesBoth([]);
    setActiveSessionId(null);
    setSidebarOpen(false);
  };

  const renameSession = (id: number, current: string) => {
    setRenameTarget({ id, current });
    setRenameValue(current);
  };

  return {
    sessions,
    setSessions,
    activeSessionId,
    setActiveSessionId,
    sidebarOpen,
    setSidebarOpen,
    renameTarget,
    setRenameTarget,
    renameValue,
    setRenameValue,
    loadSessions,
    selectSession,
    deleteSession,
    handleNewChat,
    renameSession,
  };
}
