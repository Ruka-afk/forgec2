import { useCallback, useRef } from "react";

/**
 * Tracks whether a resource has ever produced data, so a BACKGROUND refresh
 * does not flip a populated view back to its loading state.
 *
 * useApiResource applies this rule internally through its own has-data ref.
 * Hand-rolled loaders that also subscribe to WebSocket events (builds, the
 * credential vault) set `loading` on every refresh, so each event replaced the
 * rendered rows with a skeleton and made the page flash.
 */
export function useRefreshGate() {
  const hasDataRef = useRef(false);

  /**
   * True when the caller should enter its loading state. Pass `force` for a
   * user-initiated reload (e.g. changing a filter), which should always show
   * progress even when data is already on screen; background refreshes stay
   * silent.
   */
  const beginRefresh = useCallback((force = false) => force || !hasDataRef.current, []);

  /** Call after a successful load so later refreshes stay silent. */
  const markLoaded = useCallback(() => {
    hasDataRef.current = true;
  }, []);

  return { beginRefresh, markLoaded };
}
