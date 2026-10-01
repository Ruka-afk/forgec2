import { useEffect } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

const NEW_DRAFT_KEY = "forgec2.ai.newDraft";

/** Debounced, session-isolated draft persistence for the AI composer. */
export function useAISessionDraft(sessionId: number | null, input: string) {
  useEffect(() => {
    const timer = window.setTimeout(() => {
      if (sessionId == null) {
        try { window.sessionStorage.setItem(NEW_DRAFT_KEY, input); } catch { /* optional browser storage */ }
        return;
      }
      void api.putJson(paths.ai.session(sessionId), { draft: input }).catch(() => {
        // Keep typing responsive; the next edit retries the encrypted save.
      });
    }, 700);
    return () => window.clearTimeout(timer);
  }, [sessionId, input]);
}

export function readNewAISessionDraft(): string {
  try { return window.sessionStorage.getItem(NEW_DRAFT_KEY) || ""; } catch { return ""; }
}

export function clearNewAISessionDraft() {
  try { window.sessionStorage.removeItem(NEW_DRAFT_KEY); } catch { /* optional browser storage */ }
}
