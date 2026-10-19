import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useErrorToast } from "@/lib/hooks/useErrorToast";
import { useMutation } from "@/lib/hooks/useMutation";

export function useAgentNotes(agentId: string, reloadDetail: () => Promise<void> | void) {
  const { toastError } = useErrorToast();
  const [editing, setEditing] = useState(false);
  const [tags, setTags] = useState("");
  const [notes, setNotes] = useState("");
  const { mutate: saveNote, isPending: saving, reset: resetSave } = useMutation({
    fn: () => api.post(paths.agents.note(agentId), { notes, tags }),
    onSuccess: () => {
      setEditing(false);
      // useAgentDetail never rejects; keep the reload off the promise chain.
      void Promise.resolve(reloadDetail());
    },
    onError: (e) => toastError(e, "agents.detail_notes_save_failed"),
  });

  // Abandon in-progress edits when switching agents so unsaved drafts
  // never get written to the wrong implant — and so a save still in flight
  // for the previous agent cannot close the new agent's editor or reload it.
  useEffect(() => {
    setEditing(false);
    setTags("");
    setNotes("");
    resetSave();
  }, [agentId, resetSave]);

  const startEditing = useCallback((currentTags: string, currentNotes: string) => {
    setTags(currentTags);
    setNotes(currentNotes);
    setEditing(true);
  }, []);

  const cancelEditing = useCallback(() => setEditing(false), []);

  const save = useCallback(() => {
    if (!agentId) return Promise.resolve();
    return saveNote();
  }, [agentId, saveNote]);

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
