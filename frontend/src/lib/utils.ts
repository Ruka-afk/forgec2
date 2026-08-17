import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatTime(iso?: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '—'
  return d.toLocaleString()
}

/** Local wall-clock time for "last update" indicators. */
export function nowTime(): string {
  return new Date().toLocaleTimeString()
}

export function formatSize(bytes: number | undefined | null): string {
  if (!bytes) return "-"
  const units = ["B", "KB", "MB", "GB"]
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

/** Format a byte count (numbers or numeric strings). null/undefined/"" -> "-",
 *  0 -> "0 B". Sizes at the B level stay integral. */
export function formatBytes(bytes: number | string | null | undefined): string {
  if (bytes === null || bytes === undefined || bytes === "") return "-"
  const b = Number(bytes)
  if (isNaN(b) || b === 0) return "0 B"
  const k = 1024
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.min(Math.floor(Math.log(b) / Math.log(k)), units.length - 1)
  return `${(b / Math.pow(k, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function timeAgo(
  iso?: string | null,
  t?: (key: string, params?: Record<string, string | number>) => string
): string {
  if (!iso) return t?.("time.ago.unknown") ?? "—"
  const d = new Date(iso)
  if (isNaN(d.getTime())) return t?.("time.ago.unknown") ?? "—"
  const seconds = Math.floor((Date.now() - d.getTime()) / 1000)
  if (seconds < 60) return t?.("time.ago.seconds", { n: seconds }) ?? `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return t?.("time.ago.minutes", { n: minutes }) ?? `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t?.("time.ago.hours", { n: hours }) ?? `${hours}h ago`
  const days = Math.floor(hours / 24)
  return t?.("time.ago.days", { n: days }) ?? `${days}d ago`
}

/** Translate an enum-ish API value (`prefix.value`) via t(), falling back to
 *  the raw value when no key exists for it. */
export function enumLabel(
  t: (key: string, params?: Record<string, string | number>) => string,
  prefix: string,
  value: string | null | undefined
): string {
  if (!value) return ""
  const key = `${prefix}.${value}`
  const label = t(key)
  return label === key ? value : label
}
