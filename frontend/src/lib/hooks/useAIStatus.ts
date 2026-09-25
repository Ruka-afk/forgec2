import { useCallback, useEffect, useState } from "react";
import { fetchAIStatus, type AIStatus } from "@/lib/ai-status";

/**
 * Read the shared AI status without firing the request during render.
 * `status === null` means still loading; `status.known === false` means the
 * query failed, which callers must not present as "AI is disabled".
 */
export function useAIStatus(): { status: AIStatus | null; reload: () => void } {
  const [status, setStatus] = useState<AIStatus | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setStatus(null);
    void fetchAIStatus().then((st) => {
      if (!cancelled) setStatus(st);
    });
    return () => { cancelled = true; };
  }, [nonce]);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  return { status, reload };
}
