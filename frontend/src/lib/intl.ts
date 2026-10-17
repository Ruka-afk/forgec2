import { getLocale, type Locale } from "./i18n";

/**
 * Locale-aware formatting for operator-facing dates and numbers.
 *
 * The UI language — not the browser default — decides the format: an operator
 * who set the console to zh must not see an `en-US` date just because the OS
 * is English. Route every `toLocaleString`-style call through these helpers so
 * `Intl` output stays consistent with `t()`.
 *
 * `locale` is an optional override (tests, offline exports); production code
 * normally omits it and inherits the active UI language.
 */
const TAGS: Record<Locale, string> = { en: "en-US", zh: "zh-CN" };

/** BCP-47 tag for the active (or given) UI locale. */
export function intlTag(locale?: Locale): string {
  return TAGS[locale ?? getLocale()];
}

function toDate(value: Date | string | number | null | undefined): Date | null {
  if (value === null || value === undefined || value === "") return null;
  const d = value instanceof Date ? value : new Date(value);
  return Number.isNaN(d.getTime()) ? null : d;
}

/** Date + time (`10/17/2026, 2:05:06 PM` / `2026/10/17 14:05:06`); invalid input degrades to "-". */
export function formatDateTime(value: Date | string | number | null | undefined = new Date(), locale?: Locale): string {
  const d = toDate(value);
  return d ? d.toLocaleString(intlTag(locale)) : "-";
}

/** Date only (`10/17/2026` / `2026/10/17`). */
export function formatDateOnly(value: Date | string | number | null | undefined = new Date(), locale?: Locale): string {
  const d = toDate(value);
  return d ? d.toLocaleDateString(intlTag(locale)) : "-";
}

/** Time only; defaults to now (for "last updated" stamps). */
export function formatTimeOnly(
  value: Date | string | number | null | undefined = new Date(),
  opts?: Intl.DateTimeFormatOptions,
  locale?: Locale
): string {
  const d = toDate(value);
  return d ? d.toLocaleTimeString(intlTag(locale), opts) : "-";
}

/** Grouped/rounded number via the UI locale. */
export function formatNumber(value: number, opts?: Intl.NumberFormatOptions, locale?: Locale): string {
  return value.toLocaleString(intlTag(locale), opts);
}
