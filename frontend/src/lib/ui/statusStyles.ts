"use client";

/**
 * Single source of truth for semantic status/accent colors.
 *
 * All components must derive their status/tone/accent classes from here so
 * the palette tracks the design-system tokens (--success/--warning/--info/
 * --destructive/--chart-*) and dark mode never drifts from the theme.
 * Do NOT hardcode raw Tailwind colors (emerald-500, amber-600, ...) elsewhere.
 */

export type Tone = "success" | "warning" | "info" | "destructive" | "muted" | "primary";

export interface ToneStyle {
  dot: string;
  bg: string;
  text: string;
  ring: string;
}

export const toneStyles: Record<Tone, ToneStyle> = {
  success: {
    dot: "bg-success",
    bg: "bg-success/10",
    text: "text-success",
    ring: "ring-success/30",
  },
  warning: {
    dot: "bg-warning",
    bg: "bg-warning/10",
    text: "text-warning",
    ring: "ring-warning/30",
  },
  info: {
    dot: "bg-info",
    bg: "bg-info/10",
    text: "text-info",
    ring: "ring-info/30",
  },
  destructive: {
    dot: "bg-destructive",
    bg: "bg-destructive/10",
    text: "text-destructive",
    ring: "ring-destructive/30",
  },
  muted: {
    dot: "bg-muted-foreground",
    bg: "bg-muted/50",
    text: "text-muted-foreground",
    ring: "ring-muted-foreground/30",
  },
  primary: {
    dot: "bg-primary",
    bg: "bg-primary/10",
    text: "text-primary",
    ring: "ring-primary/30",
  },
};

/** Maps every status string to a semantic tone. */
export const statusTones: Record<string, Tone> = {
  online: "success",
  connected: "success",
  active: "success",
  completed: "success",
  healthy: "success",
  offline: "muted",
  pending: "muted",
  cancelled: "muted",
  inactive: "muted",
  unknown: "muted",
  stale: "warning",
  pending_approval: "warning",
  reconnecting: "warning",
  warning: "warning",
  unstable: "warning",
  running: "info",
  failed: "destructive",
  disconnected: "destructive",
  locked: "destructive",
  error: "destructive",
  burned: "destructive",
};

export function toneForStatus(status: string): Tone {
  return statusTones[status] ?? "muted";
}

/**
 * Categorical accent hues (feature icons, task-type chips, stat cards).
 * Uses the chart token ramp so categorical colors are theme-aware without
 * needing per-hue foreground pairs.
 */
export type Hue =
  | "primary"
  | "success"
  | "warning"
  | "info"
  | "destructive"
  | "muted"
  | "emerald"
  | "cyan"
  | "indigo"
  | "amber"
  | "rose"
  | "violet";

export interface HueStyle {
  text: string;
  bg: string;
  glow: string;
}

export const hueStyles: Record<Hue, HueStyle> = {
  primary: { text: "text-primary", bg: "bg-primary/10", glow: "shadow-primary/10" },
  success: { text: "text-success", bg: "bg-success/10", glow: "shadow-success/10" },
  warning: { text: "text-warning", bg: "bg-warning/10", glow: "shadow-warning/10" },
  info: { text: "text-info", bg: "bg-info/10", glow: "shadow-info/10" },
  destructive: { text: "text-destructive", bg: "bg-destructive/10", glow: "shadow-destructive/10" },
  muted: { text: "text-muted-foreground", bg: "bg-muted/50", glow: "shadow-muted-foreground/10" },
  emerald: { text: "text-chart-1", bg: "bg-chart-1/10", glow: "shadow-chart-1/10" },
  cyan: { text: "text-chart-2", bg: "bg-chart-2/10", glow: "shadow-chart-2/10" },
  indigo: { text: "text-chart-3", bg: "bg-chart-3/10", glow: "shadow-chart-3/10" },
  amber: { text: "text-chart-4", bg: "bg-chart-4/10", glow: "shadow-chart-4/10" },
  rose: { text: "text-chart-5", bg: "bg-chart-5/10", glow: "shadow-chart-5/10" },
  violet: { text: "text-chart-6", bg: "bg-chart-6/10", glow: "shadow-chart-6/10" },
};

/** Legacy hue names kept as aliases for compatibility. */
export const hueAliases: Record<string, Hue> = {
  red: "destructive",
  blue: "info",
  purple: "violet",
  green: "success",
};

export function resolveHue(color: string): Hue {
  if (color in hueStyles) return color as Hue;
  return hueAliases[color] ?? "primary";
}

/** Task-type → accent hue, used by task lists and chips. */
export const taskTypeHues: Record<string, Hue> = {
  shell: "primary",
  screenshot: "info",
  ps: "muted",
  kill: "destructive",
  hashdump: "warning",
  creds_dump: "warning",
  privesc_check: "success",
  clipboard_get: "info",
  keylogger_start: "violet",
  keylogger_dump: "violet",
  keylogger_stop: "violet",
  upload: "cyan",
  download: "cyan",
  ls: "indigo",
  sleep: "indigo",
};

export function hueForTaskType(type: string): Hue {
  return taskTypeHues[type] ?? "muted";
}