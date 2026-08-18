// Single source of truth for app navigation. The sidebar renders sections
// from here, and the breadcrumb derives segment labels from the same hrefs,
// so a page added here is automatically labelled everywhere.
import type { LucideIcon } from "lucide-react";
import {
  Activity, Bot, Shield, Fish, Zap, Bug, Tags, Layers, Wand2, Clock,
  MessageSquare, GitBranch, Link as LinkIcon, Boxes,
  Radio, Server, Cloud, Box, Wrench, Code, Key,
  Route, IdCard, Archive, SatelliteDish, ArrowLeftRight,
  FileCode, Globe, Puzzle, Network, Crosshair, ClipboardList,
  Plug, Users, Settings, Search,
} from "lucide-react";

export interface NavItemDef {
  href: string;
  labelKey: string;
  icon: LucideIcon;
  badge?: "agents" | "listeners";
  /** When false, omitted from the sidebar (still in Ctrl+K). */
  sidebar?: boolean;
}

export interface NavSectionDef {
  titleKey: string;
  /** Always visible in the sidebar; cannot be collapsed. */
  pinned?: boolean;
  /** When false, omitted from the sidebar (still in Ctrl+K and breadcrumbs). */
  sidebar?: boolean;
  items: NavItemDef[];
}

/** Primary console: 8 items. Everything else stays reachable via More + Ctrl+K. */
export const NAV_SECTIONS: NavSectionDef[] = [
  {
    titleKey: "operations",
    pinned: true,
    items: [
      { href: "/dashboard", labelKey: "nav.dashboard", icon: Activity },
      { href: "/agents", labelKey: "nav.beacons", icon: Bug, badge: "agents" },
      { href: "/listeners", labelKey: "nav.listeners", icon: Radio, badge: "listeners" },
      { href: "/generate", labelKey: "nav.generate", icon: Boxes },
      { href: "/loot", labelKey: "nav.loot", icon: Archive },
      { href: "/credentials", labelKey: "nav.credentials", icon: Key },
      { href: "/timeline", labelKey: "nav.events", icon: Clock },
      { href: "/settings", labelKey: "nav.settings", icon: Settings },
    ],
  },
  {
    titleKey: "build-deploy",
    items: [
      { href: "/dns", labelKey: "nav.dns", icon: Network },
      { href: "/infrastructure", labelKey: "nav.infrastructure", icon: Server },
      { href: "/domain-fronting", labelKey: "nav.domain_fronting", icon: Cloud },
    ],
  },
  {
    titleKey: "post-exploitation",
    items: [
      { href: "/automation", labelKey: "nav.automation", icon: Bot },
      { href: "/bof", labelKey: "nav.bof", icon: FileCode },
      { href: "/plugins", labelKey: "nav.plugins", icon: Puzzle },
      { href: "/opsec", labelKey: "nav.opsec", icon: Shield, sidebar: false },
      { href: "/lateral", labelKey: "nav.lateral", icon: ArrowLeftRight, sidebar: false },
      { href: "/privesc", labelKey: "nav.privesc", icon: Shield, sidebar: false },
      { href: "/pivoting", labelKey: "nav.pivoting", icon: Route, sidebar: false },
      { href: "/tokens", labelKey: "nav.token_store", icon: IdCard, sidebar: false },
      { href: "/scanner", labelKey: "nav.scanner", icon: SatelliteDish, sidebar: false },
      { href: "/scripting", labelKey: "nav.scripting", icon: Code, sidebar: false },
      { href: "/toolkit", labelKey: "nav.toolkit", icon: Wrench, sidebar: false },
      { href: "/password-spray", labelKey: "nav.password_spray", icon: Shield, sidebar: false },
    ],
  },
  {
    titleKey: "intel-analysis",
    items: [
      { href: "/search", labelKey: "nav.search", icon: Search },
      { href: "/audit", labelKey: "nav.audit", icon: Shield },
      { href: "/traffic", labelKey: "nav.traffic", icon: Network },
      { href: "/report", labelKey: "nav.report", icon: ClipboardList },
      { href: "/ai", labelKey: "nav.ai", icon: Bot },
      { href: "/integrations", labelKey: "nav.integrations", icon: Plug },
      { href: "/campaign", labelKey: "nav.campaign", icon: Crosshair, sidebar: false },
      { href: "/attack", labelKey: "nav.attack", icon: Shield, sidebar: false },
      { href: "/bloodhound", labelKey: "nav.bloodhound", icon: Network, sidebar: false },
      { href: "/chat", labelKey: "nav.chat", icon: MessageSquare, sidebar: false },
    ],
  },
  {
    titleKey: "lab",
    sidebar: false,
    items: [
      { href: "/phishing", labelKey: "nav.phishing", icon: Fish },
      { href: "/circuit-breaker", labelKey: "nav.circuit_breaker", icon: Zap },
      { href: "/chrome", labelKey: "nav.chrome_c2", icon: Globe },
      { href: "/cloud", labelKey: "nav.cloud", icon: Cloud },
      { href: "/ntlm", labelKey: "nav.ntlm", icon: Zap },
      { href: "/container", labelKey: "nav.container", icon: Box },
      { href: "/topology", labelKey: "nav.topology", icon: GitBranch },
      { href: "/chain", labelKey: "nav.chain", icon: LinkIcon },
    ],
  },
  {
    titleKey: "system",
    sidebar: false,
    items: [
      { href: "/users", labelKey: "nav.users", icon: Users },
      { href: "/roles", labelKey: "nav.roles", icon: Shield },
      { href: "/tags", labelKey: "nav.tags", icon: Tags },
      { href: "/groups", labelKey: "nav.groups", icon: Layers },
      { href: "/autotag", labelKey: "nav.autotag", icon: Wand2 },
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

/** Flat list of every top-level page. */
export const NAV_ITEMS: NavItemDef[] = NAV_SECTIONS.flatMap((s) => s.items);

/** href -> labelKey for every top-level page. */
export const NAV_BY_HREF: Record<string, string> = Object.fromEntries(
  NAV_ITEMS.map((i) => [i.href, i.labelKey]),
);

/** breadcrumb segment (dashes normalized to underscores) -> labelKey. */
export const NAV_SEGMENT_LABELS: Record<string, string> = Object.fromEntries(
  NAV_ITEMS.map((i) => [i.href.replace(/^\//, "").replace(/-/g, "_"), i.labelKey]),
);

/** Sub-route segment -> labelKey for per-page document.title. */
export const SUB_ROUTE_TITLE_KEYS: Record<string, string> = {
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