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
  "agents.timeline_kind_", // agent timeline event kind: t(`agents.timeline_kind_${kind}`)
  "agents.recon_", // recon section labels via RECON_LABEL_KEYS record lookup
  "agents.col_", // agents table headers via sortableHead(field, labelKey) indirection
  "agents.files_col_", // files table headers via sortHead(key, labelKey) indirection
  "agents.proc_", // process actions via t(`agents.proc_${action}`) template
  "ai.risk_", // AI suggestion risk badge: t(`ai.risk_${suggestion.risk}`)
  "events.source_", // merged timeline source label selected before t(labelKey)
];
