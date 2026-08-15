/**
 * Theme-aware categorical color helpers.
 *
 * Charts previously hardcoded hex palettes (colors.ts) that ignored the
 * design-system tokens and dark mode. These helpers resolve to the
 * `--chart-1..6` CSS variables so every categorical color tracks the theme.
 */

export const MITRE_PHASE_ORDER = [
  "Reconnaissance",
  "Resource Development",
  "Initial Access",
  "Execution",
  "Persistence",
  "Privilege Escalation",
  "Defense Evasion",
  "Credential Access",
  "Discovery",
  "Lateral Movement",
  "Collection",
  "Command and Control",
  "Exfiltration",
  "Impact",
];

function hashString(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
  return Math.abs(h);
}

/** Map a MITRE ATT&CK tactic/phase to a theme-aware chart token. */
export function phaseColor(phase: string): string {
  const idx = MITRE_PHASE_ORDER.indexOf(phase);
  const n = (idx >= 0 ? idx : hashString(phase)) % 6;
  return `var(--chart-${n + 1})`;
}

/** OS-name → theme-aware chart token (Windows=blue, Linux=amber, macOS=violet). */
export function osColor(os: string): string {
  switch (os) {
    case "Windows":
      return "var(--chart-2)";
    case "Linux":
      return "var(--chart-4)";
    case "macOS":
    case "darwin":
      return "var(--chart-6)";
    default:
      return "var(--chart-3)";
  }
}
