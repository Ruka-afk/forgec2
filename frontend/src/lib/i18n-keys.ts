// i18n-keys.ts
// Frontend-side registry for i18n dynamic key families.
//
// The BUILD-TIME single source of truth is `scripts/i18n-dynamic.mjs`
// (consumed by `scripts/check-i18n.mjs` to exclude these from the
// dead-key scan). Keep the two lists in sync.

export const DYNAMIC_KEY_PREFIXES = [
  "nav.",
  "topbar.",
  "section.",
  "settings.",
] as const;

export type DynamicKeyPrefix = (typeof DYNAMIC_KEY_PREFIXES)[number];

// Build a dynamic i18n key with a known prefix. Prefer this over raw string
// concatenation so call sites stay typed and the prefix set is enforced.
export function dynamicKey(prefix: DynamicKeyPrefix, id: string): string {
  return `${prefix}${id}`;
}
