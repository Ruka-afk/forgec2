"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";

export function useAgentNotes(agentId: string, reloadDetail: () => Promise<void> | void, errorMessage: string) {
  const [editing, setEditing] = useState(false);
  const [tags, setTags] = useState("");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  // Abandon in-progress edits when switching agents so unsaved drafts
  // never get written to the wrong implant.
  useEffect(() => {
    setEditing(false);
    setTags("");
    setNotes("");
    setSaving(false);
  }, [agentId]);

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
      await api.post(paths.agents.note(agentId), { notes, tags });
      if (!mountedRef.current) return;
      setEditing(false);
      await Promise.resolve(reloadDetail());
    } catch {
      if (mountedRef.current) toast.error(errorMessage);
    } finally {
      if (mountedRef.current) setSaving(false);
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
