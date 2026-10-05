import { useMemo } from "react";
import { type VaultEntry } from "./types";

/**
 * Filter the credential vault by the page's active search/filter state.
 *
 * Lives in its own hook (instead of inline in the page) because the page
 * component's other values are not preservable by the React compiler trace;
 * an inline manual memo of this filter would trip
 * react-hooks/preserve-manual-memoization. Inside a small hook the memo
 * traces cleanly and is preserved: recomputed only when the vault or one of
 * the four filter states actually changes.
 */
export function useFilteredCredentials(
  vaultEntries: VaultEntry[],
  searchQuery: string,
  typeFilter: string,
  confirmedFilter: string,
  lifecycleFilter: string,
): VaultEntry[] {
  return useMemo(
    () =>
      vaultEntries.filter((entry) => {
        if (searchQuery) {
          const q = searchQuery.toLowerCase();
          if (
            !entry.username.toLowerCase().includes(q) &&
            !entry.domain?.toLowerCase().includes(q) &&
            !entry.notes?.toLowerCase().includes(q)
          ) return false;
        }
        if (typeFilter !== "all" && entry.type !== typeFilter) return false;
        if (confirmedFilter === "true" && !entry.confirmed) return false;
        if (confirmedFilter === "false" && entry.confirmed) return false;
        if (lifecycleFilter && entry.lifecycle !== lifecycleFilter) return false;
        return true;
      }),
    [vaultEntries, searchQuery, typeFilter, confirmedFilter, lifecycleFilter],
  );
}
