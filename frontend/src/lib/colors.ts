import { MITRE_PHASE_ORDER, osColor, phaseColor } from "@/lib/chart-palette";

/**
 * Deprecated: these maps now resolve to theme-aware `--chart-*` CSS variables
 * instead of hardcoded hex. Prefer importing `phaseColor` / `osColor`
 * directly from `@/lib/chart-palette`.
 */
export const MITRE_PHASE_COLORS: Record<string, string> = Object.fromEntries(
  MITRE_PHASE_ORDER.map((phase) => [phase, phaseColor(phase)])
);

export const OS_CHART_COLORS: Record<string, string> = {
  Windows: osColor("Windows"),
  Linux: osColor("Linux"),
  macOS: osColor("macOS"),
  darwin: osColor("darwin"),
};
