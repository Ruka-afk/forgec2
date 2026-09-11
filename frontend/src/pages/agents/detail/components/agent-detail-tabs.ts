// Agent detail Tab model + capability gating.
//
// The detail page groups its 15+ sections into tabs (see AGENT_DETAIL_TABS).
// C implants only support a subset — previously scattered as inline `!isC`
// guards across AgentDetailPage. sectionSupported() centralizes that matrix
// so new sections declare support in one place.

export type AgentDetailTabId =
  | "overview"
  | "tasks"
  | "recon"
  | "evasion"
  | "collect"
  | "timeline";

export interface AgentDetailTab {
  id: AgentDetailTabId;
  /** i18n key for the tab label */
  labelKey: string;
}

export const AGENT_DETAIL_TABS: AgentDetailTab[] = [
  { id: "overview", labelKey: "agents.detail_tab_overview" },
  { id: "tasks", labelKey: "agents.detail_tab_tasks" },
  { id: "recon", labelKey: "agents.detail_tab_recon" },
  { id: "evasion", labelKey: "agents.detail_tab_evasion" },
  { id: "collect", labelKey: "agents.detail_tab_collect" },
  { id: "timeline", labelKey: "agents.detail_tab_timeline" },
];

export function isAgentDetailTabId(v: string | null | undefined): v is AgentDetailTabId {
  return AGENT_DETAIL_TABS.some((t) => t.id === v);
}

/** Section keys rendered inside the detail tabs. */
export type AgentDetailSection =
  | "diagnose"
  | "tasks"
  | "hostinfo"
  | "recon"
  | "process"
  | "evasion"
  | "inject"
  | "screenshots"
  | "screentrigger"
  | "registry"
  | "browserhistory"
  | "keylogger"
  | "clipboard"
  | "wallpaper"
  | "webcammic"
  | "timeline";

/** Which tab hosts each section. */
export const AGENT_DETAIL_SECTION_TABS: Record<AgentDetailSection, AgentDetailTabId> = {
  diagnose: "overview",
  tasks: "tasks",
  hostinfo: "overview",
  recon: "recon",
  process: "recon",
  evasion: "evasion",
  inject: "evasion",
  screenshots: "collect",
  screentrigger: "collect",
  registry: "collect",
  browserhistory: "collect",
  keylogger: "collect",
  clipboard: "collect",
  wallpaper: "collect",
  webcammic: "collect",
  timeline: "timeline",
};

/**
 * Whether a section can run on the given implant. C implants only support
 * the core subset; everything else renders gated with the shared notice.
 */
export function sectionSupported(
  section: AgentDetailSection,
  opts: { isCImplant: boolean },
): boolean {
  if (!opts.isCImplant) return true;
  switch (section) {
    case "diagnose":
    case "tasks":
    case "hostinfo":
    case "process":
    case "timeline":
      return true;
    default:
      return false;
  }
}
