// Single source of truth for app navigation. The sidebar renders sections
// from here, and the breadcrumb derives segment labels from the same hrefs,
// so a page added here is automatically labelled everywhere.
import type { LucideIcon } from "lucide-react";
import {
  Activity, Bot, Shield, Fish, Zap, Bug, Tags, Layers, Wand2, Clock,
  FolderTree, Bell, MessageSquare, GitBranch, Link as LinkIcon, Boxes,
  Radio, Hammer, Server, Cloud, PenTool, Box, Wrench, Code, Key,
  Route, IdCard, Archive, SatelliteDish, ArrowLeftRight,
  FileCode, Globe, Puzzle, Network, Crosshair, ClipboardList,
  Plug, Users, Settings, Search,
} from "lucide-react";

export interface NavItemDef {
  href: string;
  labelKey: string;
  icon: LucideIcon;
  badge?: "agents" | "listeners";
}

export interface NavSectionDef {
  titleKey: string;
  items: NavItemDef[];
}

export const NAV_SECTIONS: NavSectionDef[] = [
  {
    titleKey: "operations",
    items: [
      { href: "/dashboard", labelKey: "nav.dashboard", icon: Activity },
      { href: "/agents", labelKey: "nav.beacons", icon: Bug, badge: "agents" },
      { href: "/tasks", labelKey: "nav.tasks", icon: Clock },
      { href: "/timeline", labelKey: "nav.timeline", icon: Clock },
      { href: "/files", labelKey: "nav.files", icon: FolderTree },
      { href: "/notifications", labelKey: "nav.notifications", icon: Bell },
      { href: "/search", labelKey: "nav.search", icon: Search },
      { href: "/automation", labelKey: "nav.automation", icon: Bot },
      { href: "/opsec", labelKey: "nav.opsec", icon: Shield },
    ],
  },
  {
    titleKey: "build-deploy",
    items: [
      { href: "/generate", labelKey: "nav.generate", icon: Boxes },
      { href: "/listeners", labelKey: "nav.listeners", icon: Radio, badge: "listeners" },
      { href: "/builds", labelKey: "nav.builds", icon: Hammer },
      { href: "/profiles", labelKey: "nav.profiles", icon: PenTool },
      { href: "/dns", labelKey: "nav.dns", icon: Network },
      { href: "/infrastructure", labelKey: "nav.infrastructure", icon: Server },
      { href: "/domain-fronting", labelKey: "nav.domain_fronting", icon: Cloud },
      { href: "/packer", labelKey: "nav.packer", icon: Box },
      { href: "/stager", labelKey: "nav.stager", icon: GitBranch },
    ],
  },
  {
    titleKey: "post-exploitation",
    items: [
      { href: "/credentials", labelKey: "nav.credentials", icon: Key },
      { href: "/loot", labelKey: "nav.loot", icon: Archive },
      { href: "/lateral", labelKey: "nav.lateral", icon: ArrowLeftRight },
      { href: "/privesc", labelKey: "nav.privesc", icon: Shield },
      { href: "/pivoting", labelKey: "nav.pivoting", icon: Route },
      { href: "/tokens", labelKey: "nav.token_store", icon: IdCard },
      { href: "/scanner", labelKey: "nav.scanner", icon: SatelliteDish },
      { href: "/bof", labelKey: "nav.bof", icon: FileCode },
      { href: "/scripting", labelKey: "nav.scripting", icon: Code },
      { href: "/toolkit", labelKey: "nav.toolkit", icon: Wrench },
      { href: "/plugins", labelKey: "nav.plugins", icon: Puzzle },
    ],
  },
  {
    titleKey: "intel-analysis",
    items: [
      { href: "/audit", labelKey: "nav.audit", icon: Shield },
      { href: "/traffic", labelKey: "nav.traffic", icon: Network },
      { href: "/campaign", labelKey: "nav.campaign", icon: Crosshair },
      { href: "/attack", labelKey: "nav.attack", icon: Shield },
      { href: "/report", labelKey: "nav.report", icon: ClipboardList },
      { href: "/ai", labelKey: "nav.ai", icon: Bot },
      { href: "/integrations", labelKey: "nav.integrations", icon: Plug },
      { href: "/bloodhound", labelKey: "nav.bloodhound", icon: Network },
      { href: "/chat", labelKey: "nav.chat", icon: MessageSquare },
    ],
  },
  {
    titleKey: "lab",
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
    items: [
      { href: "/settings", labelKey: "nav.settings", icon: Settings },
      { href: "/users", labelKey: "nav.users", icon: Users },
      { href: "/roles", labelKey: "nav.roles", icon: Shield },
      { href: "/tags", labelKey: "nav.tags", icon: Tags },
      { href: "/groups", labelKey: "nav.groups", icon: Layers },
      { href: "/autotag", labelKey: "nav.autotag", icon: Wand2 },
    ],
  },
];

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