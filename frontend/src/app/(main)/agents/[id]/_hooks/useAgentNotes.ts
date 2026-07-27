"use client";

import { useCallback, useState } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";

export function useAgentNotes(agentId: string, reloadDetail: () => Promise<void> | void, errorMessage: string) {
  const [editing, setEditing] = useState(false);
  const [tags, setTags] = useState("");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);

  const startEditing = useCallback((currentTags: string, currentNotes: string) => {
    setTags(currentTags);
    setNotes(currentNotes);
    setEditing(true);
  }, []);

  const cancelEditing = useCallback(() => setEditing(false), []);

  const save = useCallback(async () => {
    if (!agentId) return;
    setSaving(true);
    try {
      await api.post(`/agents/${agentId}/note`, { notes, tags });
      setEditing(false);
      await Promise.resolve(reloadDetail());
    } catch {
      toast.error(errorMessage);
    } finally {
      setSaving(false);
    }
  }, [agentId, notes, tags, reloadDetail, errorMessage]);

  return {
    editing,
    setEditing,
    tags,
    setTags,
    notes,
    setNotes,
    saving,
    startEditing,
    cancelEditing,
    save,
  };
}
