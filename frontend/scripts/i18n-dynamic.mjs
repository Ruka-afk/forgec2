// i18n-dynamic.mjs
// Single source of truth for i18n key families that are resolved
// DYNAMICALLY at runtime (built via t(key) over a computed prefix).
//
// Because these keys are not present as literal t("x.y") calls, the
// check-i18n.mjs dead-key scan must NOT flag them as unused.
// Keep this list in ONE place and import it from check-i18n.mjs
// (the frontend uses runtime prefixes via SHARED_DYNAMIC_PREFIXES).
export const DYNAMIC_PREFIXES = [
  "nav.",       // sidebar nav items: t(navKey) over the nav config
  "topbar.",    // topbar dropdown labels
  "section.",   // settings / section headers
  "settings.",  // settings form labels
  "auto.type_", // automation alert-rule types: t(`auto.type_${r.type}`)
  "notifications.severity_", // notification badge severities: t(`notifications.severity_${n.severity}`)
  "search.type_", // search result types: t(`search.type_${r.type}`)
  "generate.format_", // payload format picker: t(PAYLOAD_FORMAT_LABEL[key])
];
