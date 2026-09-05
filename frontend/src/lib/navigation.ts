// Single source of truth for app navigation. The sidebar renders sections
// from here, and the breadcrumb derives segment labels from the same hrefs,
// so a page added here is automatically labelled everywhere.
import type { LucideIcon } from "lucide-react";
import type { PermissionKey } from "./permission-keys";
import {
  Activity, Bot, Shield, Fish, Zap, Bug, Tags, Layers, Wand2, Clock,
  MessageSquare, GitBranch, Link as LinkIcon, Boxes,
  Radio, Server, Cloud, Box, Wrench, Code, Key,
  Route, IdCard, Archive, SatelliteDish, ArrowLeftRight,
  FileCode, Puzzle, Network, Crosshair, ClipboardList,
  Plug, Users, Settings, ListOrdered,
} from "lucide-react";

interface NavItemDef {
  href: string;
  labelKey: string;
  icon: LucideIcon;
  badge?: "agents" | "listeners";
  /** When false, omitted from the sidebar (still in Ctrl+K). */
  sidebar?: boolean;
  /**
   * Any-of permission requirement, mirroring per-route enforcement in
   * internal/server/routes.go. Items without perms are visible to every
   * authenticated operator. UI-only: backend stays authoritative.
   */
  perms?: PermissionKey[];
  /** Page shell contract used by AppLayout and page-level containers. */
  layout?: "standard" | "wide" | "workspace";
}

interface NavSectionDef {
  titleKey: string;
  /** Always visible in the sidebar; cannot be collapsed. */
  pinned?: boolean;
  /** When false, omitted from the sidebar (still in Ctrl+K and breadcrumbs). */
  sidebar?: boolean;
  items: NavItemDef[];
}

/** Light-first command-center navigation. Low-frequency routes remain in More + Ctrl+K. */
export const NAV_SECTIONS: NavSectionDef[] = [
  {
    titleKey: "operations",
    pinned: true,
    items: [
      { href: "/dashboard", labelKey: "nav.dashboard", icon: Activity, perms: ["agents.read"] },
      { href: "/agents", labelKey: "nav.beacons", icon: Bug, badge: "agents", perms: ["agents.read"] },
      { href: "/listeners", labelKey: "nav.listeners", icon: Radio, badge: "listeners", perms: ["listeners.read"] },
      { href: "/timeline", labelKey: "nav.events", icon: Clock, perms: ["agents.read"] },
    ],
  },
  {
    titleKey: "build-deploy",
    items: [
      { href: "/generate", labelKey: "nav.generate", icon: Boxes, perms: ["agents.read"] },
      { href: "/infrastructure", labelKey: "nav.infrastructure", icon: Server },
      { href: "/dns", labelKey: "nav.dns", icon: Network },
      { href: "/domain-fronting", labelKey: "nav.domain_fronting", icon: Cloud, perms: ["agents.read"] },
    ],
  },
  {
    titleKey: "post-exploitation",
    items: [
      { href: "/automation", labelKey: "nav.automation", icon: Bot, perms: ["automation.read"] },
      { href: "/macros", labelKey: "nav.macros", icon: ListOrdered, perms: ["agents.read"] },
      { href: "/bof", labelKey: "nav.bof", icon: FileCode, perms: ["agents.read"] },
      { href: "/plugins", labelKey: "nav.plugins", icon: Puzzle, perms: ["plugins.read"] },
      { href: "/opsec", labelKey: "nav.opsec", icon: Shield, sidebar: false, perms: ["opsec.read"] },
      { href: "/lateral", labelKey: "nav.lateral", icon: ArrowLeftRight, sidebar: false, perms: ["agents.read"] },
      { href: "/privesc", labelKey: "nav.privesc", icon: Shield, sidebar: false, perms: ["agents.read"] },
      { href: "/pivoting", labelKey: "nav.pivoting", icon: Route, sidebar: false, perms: ["agents.read"] },
      { href: "/tokens", labelKey: "nav.token_store", icon: IdCard, sidebar: false, perms: ["settings.read"] },
      { href: "/scanner", labelKey: "nav.scanner", icon: SatelliteDish, sidebar: false, perms: ["agents.read"] },
      { href: "/scripting", labelKey: "nav.scripting", icon: Code, sidebar: false, perms: ["settings.read"] },
      { href: "/toolkit", labelKey: "nav.toolkit", icon: Wrench, sidebar: false, perms: ["agents.read"] },
      { href: "/password-spray", labelKey: "nav.password_spray", icon: Shield, sidebar: false, perms: ["agents.write"] },
    ],
  },
  {
    titleKey: "intel-analysis",
    items: [
      { href: "/loot", labelKey: "nav.loot", icon: Archive, perms: ["agents.read"] },
      { href: "/credentials", labelKey: "nav.credentials", icon: Key, perms: ["credentials.read"] },
      { href: "/audit", labelKey: "nav.audit", icon: Shield, perms: ["audit.read"] },
      { href: "/traffic", labelKey: "nav.traffic", icon: Network, perms: ["agents.read"] },
      { href: "/report", labelKey: "nav.report", icon: ClipboardList, perms: ["agents.read"] },
      { href: "/ai", labelKey: "nav.ai", icon: Bot, layout: "workspace", perms: ["settings.read"] },
      { href: "/integrations", labelKey: "nav.integrations", icon: Plug, perms: ["settings.read"] },
      { href: "/campaign", labelKey: "nav.campaign", icon: Crosshair, sidebar: false, perms: ["campaigns.read"] },
      { href: "/attack", labelKey: "nav.attack", icon: Shield, sidebar: false, perms: ["campaigns.read"] },
      { href: "/bloodhound", labelKey: "nav.bloodhound", icon: Network, sidebar: false, perms: ["intel.read"] },
      { href: "/chat", labelKey: "nav.chat", icon: MessageSquare, sidebar: false, layout: "workspace", perms: ["agents.read"] },
    ],
  },
  {
    titleKey: "lab",
    sidebar: false,
    items: [
      { href: "/phishing", labelKey: "nav.phishing", icon: Fish, perms: ["campaigns.read"] },
      { href: "/circuit-breaker", labelKey: "nav.circuit_breaker", icon: Zap, perms: ["opsec.read"] },
      { href: "/cloud", labelKey: "nav.cloud", icon: Cloud, perms: ["intel.read"] },
      { href: "/ntlm", labelKey: "nav.ntlm", icon: Zap, perms: ["agents.read"] },
      { href: "/container", labelKey: "nav.container", icon: Box, perms: ["agents.read"] },
      { href: "/topology", labelKey: "nav.topology", icon: GitBranch, perms: ["agents.read"] },
      { href: "/chain", labelKey: "nav.chain", icon: LinkIcon, perms: ["agents.read"] },
    ],
  },
  {
    titleKey: "system",
    sidebar: false,
    items: [
      { href: "/settings", labelKey: "nav.settings", icon: Settings, perms: ["settings.read"] },
      { href: "/users", labelKey: "nav.users", icon: Users, perms: ["users.read"] },
      { href: "/roles", labelKey: "nav.roles", icon: Shield, perms: ["roles.read"] },
      { href: "/tags", labelKey: "nav.tags", icon: Tags, perms: ["agents.write"] },
      { href: "/groups", labelKey: "nav.groups", icon: Layers, perms: ["groups.read"] },
      { href: "/autotag", labelKey: "nav.autotag", icon: Wand2, perms: ["settings.read"] },
    ],
  },
];

/** Sidebar sections only — Lab and System stay in Ctrl+K. */
export function sidebarNavSections(sections: NavSectionDef[] = NAV_SECTIONS): NavSectionDef[] {
  return sections
    .filter((s) => s.sidebar !== false)
    .map((s) => ({ ...s, items: s.items.filter((i) => i.sidebar !== false) }))
    .filter((s) => s.items.length > 0);
}

/**
 * Items the current operator may open: no perms requirement, or any-of the
 * held permissions. Unknown permission state (still loading) shows everything
 * — backend enforcement is authoritative.
 */
export function filterNavByPermissions(
  items: NavItemDef[],
  permissions: readonly PermissionKey[] | null | undefined,
): NavItemDef[] {
  if (!permissions) return items;
  return items.filter((i) => !i.perms || i.perms.some((p) => permissions.includes(p)));
}

/** Flat list of every top-level page. */
export const NAV_ITEMS: NavItemDef[] = NAV_SECTIONS.flatMap((s) => s.items);

/** href -> labelKey for every top-level page. */
export const NAV_BY_HREF: Record<string, string> = Object.fromEntries(
  NAV_ITEMS.map((i) => [i.href, i.labelKey]),
);

export const NAV_LAYOUT_BY_HREF: Record<string, "standard" | "wide" | "workspace"> = Object.fromEntries(
  NAV_ITEMS.map((i) => [i.href, i.layout ?? "wide"]),
);

/** breadcrumb segment (dashes normalized to underscores) -> labelKey. */
export const NAV_SEGMENT_LABELS: Record<string, string> = Object.fromEntries(
  NAV_ITEMS.map((i) => [i.href.replace(/^\//, "").replace(/-/g, "_"), i.labelKey]),
);

/** Sub-route segment -> labelKey for per-page document.title. */
const SUB_ROUTE_TITLE_KEYS: Record<string, string> = {
  config: "agents.config_title",
  shell: "agents.shell",
  files: "nav.files",
  screen: "agents.screen_title",
  "remote-desktop": "agents.rdp_title",
  token: "agents.token_title",
  traffic: "nav.traffic",
  persistence: "agents.persistence_title",
};

/** Resolve the i18n label key for a pathname (deepest named segment first).
 *  UUID/numeric segments are skipped so agent detail pages fall back to the
 *  parent section label. Returns null when no segment has a label. */
export function getPageTitleKey(pathname: string): string | null {
  const segments = pathname.split("/").filter(Boolean);
  for (let i = segments.length - 1; i >= 0; i--) {
    const seg = segments[i];
    if (seg.match(/^[0-9a-f]{8}-[0-9a-f]{4}-/i) || seg.match(/^\d+$/)) continue;
    const subKey = SUB_ROUTE_TITLE_KEYS[seg];
    if (subKey) return subKey;
    const navKey = NAV_SEGMENT_LABELS[seg] || NAV_SEGMENT_LABELS[seg.replace(/-/g, "_")];
    if (navKey) return navKey;
  }
  return null;
}
