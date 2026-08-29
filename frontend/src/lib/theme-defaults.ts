export type Theme = "light" | "dark" | "system";

/** Light-first default. Explicitly saved dark/system preferences are preserved. */
export const DEFAULT_THEME: Theme = "light";

export function resolveStoredTheme(saved: string | null | undefined): Theme {
  if (saved === "light" || saved === "dark" || saved === "system") return saved;
  return DEFAULT_THEME;
}
