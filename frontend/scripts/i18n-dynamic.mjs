// i18n-dynamic.mjs
// Single source of truth for i18n key families that are resolved
// DYNAMICALLY at runtime (built via t(key) over a computed prefix).
//
// Because these keys are not present as literal t("x.y") calls, the
// check-i18n.mjs dead-key scan must NOT flag them as unused.
// Keep this list in ONE place and import it from both the check script
// and the frontend (src/lib/i18n-keys.ts).
export const DYNAMIC_PREFIXES = [
  "nav.",       // sidebar nav items: t(navKey) over the nav config
  "topbar.",    // topbar dropdown labels
  "section.",   // settings / section headers
  "settings.",  // settings form labels
];
