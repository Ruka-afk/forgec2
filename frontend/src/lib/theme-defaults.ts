export type Theme = "light" | "dark" | "system";

/** Night-ops default. First visit and missing storage resolve to dark. */
export const DEFAULT_THEME: Theme = "dark";

export function resolveStoredTheme(saved: string | null | undefined): Theme {
  if (saved === "light" || saved === "dark" || saved === "system") return saved;
  return DEFAULT_THEME;
}
