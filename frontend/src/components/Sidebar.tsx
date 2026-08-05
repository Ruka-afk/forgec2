"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useWebSocket } from "@/lib/useWebSocket";
import { useEffect, useState, useCallback, memo } from "react";
import { useShallow } from "zustand/shallow";
import { useI18n } from "@/lib/i18n";
import { useAppStore } from "@/lib/store";
import { api } from "@/lib/api";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import type { DashboardStats } from "@/types/agent";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import {
  Activity, Bot, Shield, Fish, Zap, Bug, Tags, Layers, Wand2, Clock,
  FolderTree, Bell, MessageSquare, GitBranch, Link as LinkIcon, Boxes,
  Radio, Hammer, Server, Cloud, PenTool, Box, Wrench, Code, Key,
  Route, IdCard, Archive, Images, SatelliteDish, ArrowLeftRight,
  FileCode, FileText, Globe, Puzzle, Network, Crosshair, ClipboardList,
  Plug, Users, Settings, Book, ChevronDown, Search,
  type LucideIcon,
} from "lucide-react";
import {
  defaultSidebarSections as defaultSections,
  mergeSidebarSections,
  SIDEBAR_SECTIONS_KEY,
  SIDEBAR_SECTIONS_LEGACY_KEY,
} from "@/lib/sidebar-sections";

interface NavItem {
  href: string;
  labelKey: string;
  icon: LucideIcon;
  badge?: "agents" | "listeners";
}

interface NavSection {
  titleKey: string;
  items: NavItem[];
}

const navSections: NavSection[] = [
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
      { href: "/workflows", labelKey: "nav.workflows", icon: GitBranch },
      { href: "/scheduler", labelKey: "nav.scheduler", icon: Clock },
      { href: "/command_templates", labelKey: "nav.templates", icon: FileText },
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
      { href: "/screenshots", labelKey: "nav.screenshots", icon: Images },
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
      { href: "/docs", labelKey: "nav.docs", icon: Book },
    ],
  },
];

let _savedSections: Record<string, boolean> | null = null;

function getSavedSections(): Record<string, boolean> {
  if (_savedSections) return _savedSections;
  try {
    let raw = localStorage.getItem(SIDEBAR_SECTIONS_KEY);
    if (!raw) {
      raw = localStorage.getItem(SIDEBAR_SECTIONS_LEGACY_KEY);
    }
    if (!raw) return { ...defaultSections };
    const saved = JSON.parse(raw) as Record<string, boolean>;
    const merged = mergeSidebarSections(saved, defaultSections);
    _savedSections = merged;
    try {
      localStorage.setItem(SIDEBAR_SECTIONS_KEY, JSON.stringify(merged));
    } catch { /* ignore */ }
    return merged;
  } catch { /* ignore */ }
  return { ...defaultSections };
}

function saveSectionState(state: Record<string, boolean>) {
  _savedSections = state;
  try { localStorage.setItem(SIDEBAR_SECTIONS_KEY, JSON.stringify(state)); }
  catch { /* ignore */ }
}

const SidebarLogo = memo(function SidebarLogo({
  collapsed,
  onToggle,
  expandLabel,
  collapseLabel,
}: {
  collapsed: boolean;
  onToggle: () => void;
  expandLabel: string;
  collapseLabel: string;
}) {
  return (
    <div className={`border-b border-border flex items-center ${collapsed ? 'justify-center px-0 py-2' : 'px-3 py-2.5'}`}>
      <Button variant="ghost" onClick={onToggle}
        className={`flex items-center gap-x-2.5 hover:opacity-80 transition-all duration-200 ${collapsed ? 'justify-center w-full' : ''}`}
        aria-label={collapsed ? expandLabel : collapseLabel}
        aria-expanded={!collapsed}>
        <div className="w-8 h-8 bg-gradient-to-br from-primary to-primary/75 rounded-xl flex items-center justify-center shadow-lg shadow-primary/25 ring-1 ring-primary/20 shrink-0">
          <Shield className="w-4 h-4 text-primary-foreground" />
        </div>
        {!collapsed && (
          <div className="text-left leading-tight">
            <span className="font-bold text-xs tracking-tight text-foreground font-mono">Forge</span>
            <span className="font-bold text-xs tracking-tight text-primary font-mono">C2</span>
            <div className="mono-eyebrow text-muted-foreground/50">net · ops</div>
          </div>
        )}
      </Button>
    </div>
  );
});

const SidebarNav = memo(function SidebarNav({ collapsed, sections, toggleSection, pathname, stats, t, searchQuery }: {
  collapsed: boolean;
  sections: Record<string, boolean>;
  toggleSection: (key: string) => void;
  pathname: string;
  stats: DashboardStats | null;
  t: (key: string, params?: Record<string, string | number>) => string;
  searchQuery: string;
}) {
  function isActive(href: string) {
    if (href === "/dashboard") return pathname === "/dashboard";
    return pathname.startsWith(href);
  }

  const searching = searchQuery.trim().length > 0;
  const query = searching ? searchQuery.toLowerCase() : "";

  const filteredSections = searching
    ? navSections.map((section) => ({
        ...section,
        items: section.items.filter((item) =>
          t(item.labelKey).toLowerCase().includes(query)
        ),
      })).filter((section) => section.items.length > 0)
    : navSections;

  return (
    <nav className={collapsed ? 'flex flex-col items-center gap-1' : 'space-y-0 text-(--fs-body-sm)'}>
      {filteredSections.map((section, idx) => (
        <div key={section.titleKey} className={collapsed ? 'w-full' : ''}>
          {!collapsed && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => toggleSection(section.titleKey)}
              className={`w-full flex items-center gap-x-1 px-2 pt-1 pb-0.5 cursor-pointer select-none transition-colors hover:text-foreground justify-start ${idx > 0 ? 'mt-2.5 border-t border-border/40 pt-2.5' : ''}`}
            >
              <ChevronDown className={`w-3 h-3 text-muted-foreground transition-transform duration-200 ${(searching || sections[section.titleKey]) ? '' : '-rotate-90'}`} />
              <span className="mono-eyebrow text-muted-foreground/60">{t("section." + section.titleKey)}</span>
              {section.titleKey === "lab" && (
                <Badge variant="secondary" className="ml-auto px-1.5 py-px text-(--fs-micro) leading-none rounded bg-warning/15 text-warning-foreground font-medium">
                  {t("section.lab_badge")}
                </Badge>
              )}
            </Button>
          )}
          {(searching || sections[section.titleKey]) && section.items.map((item) => {
            const Icon = item.icon;
            const linkEl = (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-x-2.5 rounded-lg transition-all duration-150 hover:translate-x-0.5 ${collapsed ? 'group relative' : ''}
                  ${collapsed ? 'justify-center px-0 py-2 mx-auto w-10 h-10' : 'px-2 py-1'}
                  ${isActive(item.href)
                    ? 'bg-primary/12 text-primary font-medium border-l-2 border-primary shadow-sm'
                    : 'text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground'}`}
              >
                <span className="relative inline-flex">
                  <Icon className={collapsed ? 'w-5 h-5' : 'w-4 h-4'} />
                  {collapsed && item.badge === "agents" && stats != null && (stats.online_agents ?? 0) > 0 && (
                    <span className="absolute -top-1 -right-2 w-2.5 h-2.5 rounded-full bg-emerald-500" />
                  )}
                  {collapsed && item.badge === "listeners" && stats != null && (stats.total_listeners ?? 0) > 0 && (
                    <span className="absolute -top-1 -right-2 w-2.5 h-2.5 rounded-full bg-emerald-500" />
                  )}
                </span>
                {!collapsed && (
                  <>
                    <span className="truncate flex-1 text-xs">{t(item.labelKey)}</span>
                    {item.badge === "agents" && stats != null && (
                      <Badge variant="secondary" className="px-1.5 py-px text-(--fs-micro) leading-none rounded bg-primary/20 text-primary font-mono">
                        {stats.online_agents ?? 0}
                      </Badge>
                    )}
                    {item.badge === "listeners" && stats != null && (
                      <Badge variant="secondary" className="px-1.5 py-px text-(--fs-micro) leading-none rounded bg-primary/10 text-primary font-mono">
                        {stats.total_listeners ?? 0}
                      </Badge>
                    )}
                  </>
                )}
              </Link>
            );
            return collapsed ? (
              <Tooltip key={item.href}>
                <TooltipTrigger render={linkEl} />
                <TooltipContent side="right">{t(item.labelKey)}</TooltipContent>
              </Tooltip>
            ) : linkEl;
          })}
        </div>
      ))}
    </nav>
  );
});

const SidebarFooter = memo(function SidebarFooter({ collapsed, connected, reconnectFailed, onlineUsers, currentUsername, t }: {
  collapsed: boolean;
  connected: boolean;
  reconnectFailed: boolean;
  onlineUsers: Array<{ username: string }>;
  currentUsername: string;
  t: (key: string, params?: Record<string, string | number>) => string;
}) {
  return (
    <div className={`border-t border-border/60 ${collapsed ? 'px-2 py-2' : 'px-3 py-2.5'} space-y-2`}>
      {!collapsed && connected && (
        <div className="space-y-1">
          <div className="mono-eyebrow text-muted-foreground/50">{t("sidebar.online_operators")}</div>
          {currentUsername && (
            <div className="flex items-center gap-x-2 text-(--fs-xs-sm) text-foreground font-medium">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0 animate-pulse" />
              <span className="truncate mono-cell">{t("sidebar.current_operator", { username: currentUsername })}</span>
            </div>
          )}
          {onlineUsers.filter((u) => u.username !== currentUsername).map((u) => (
            <div key={u.username} className="flex items-center gap-x-2 text-(--fs-xs-sm) text-muted-foreground">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0" />
              <span className="truncate">{u.username}</span>
            </div>
          ))}
        </div>
      )}
      <div className="flex items-center gap-x-2 rounded-md bg-secondary/50 dark:bg-secondary/30 border border-border/40 px-2 py-1.5">
        <span className={`w-2 h-2 rounded-full ${connected ? "bg-emerald-500 animate-pulse" : reconnectFailed ? "bg-red-500" : "bg-amber-500 animate-pulse"}`} />
        <span className="mono-cell text-(--fs-micro-sm) text-muted-foreground/80">
          {connected ? t("common.live") : t("common.disconnected")}
        </span>
        {!connected && <span className={`ml-auto text-(--fs-micro) font-medium ${reconnectFailed ? "text-destructive" : "text-amber-500"}`}>{reconnectFailed ? t("sidebar.offline") : t("sidebar.reconnecting")}</span>}
      </div>
    </div>
  );
});

export default function Sidebar() {
  const pathname = usePathname();
  const { connected, reconnectFailed, subscribe } = useWebSocket();
  const { t } = useI18n();
  const [sections, setSections] = useState<Record<string, boolean>>(defaultSections);
  const [searchQuery, setSearchQuery] = useState("");
  const stats = useAppStore((s) => s.stats);
  const fetchStats = useAppStore((s) => s.fetchStats);
  const sidebarCollapsed = useAppStore((s) => s.sidebarCollapsed);
  const isMobile = useAppStore((s) => s.isMobile);
  const mobileMenuOpen = useAppStore((s) => s.mobileMenuOpen);
  const toggleSidebar = useAppStore((s) => s.toggleSidebar);
  const setMobileMenuOpen = useAppStore((s) => s.setMobileMenuOpen);
  const onlineUsers = useAppStore(useShallow((s) => s.onlineUsers));
  const setOnlineUsers = useAppStore((s) => s.setOnlineUsers);
  const currentUsername = useAppStore((s) => s.currentUsername);
  const setCurrentUsername = useAppStore((s) => s.setCurrentUsername);
  const setCurrentUserRole = useAppStore((s) => s.setCurrentUserRole);

  useEffect(() => { Promise.resolve().then(() => setSections(getSavedSections())); }, []);

  useEffect(() => {
    if (currentUsername) return;
    api.get<{ CurrentUsername?: string; CurrentUserRole?: string }>("/settings").then((d) => {
      if (d.CurrentUsername) setCurrentUsername(d.CurrentUsername);
      if (d.CurrentUserRole) setCurrentUserRole(d.CurrentUserRole);
    }).catch((e) => { if (process.env.NODE_ENV === "development") console.error("Sidebar: failed to fetch username", e); });
  }, [currentUsername, setCurrentUsername, setCurrentUserRole]);

  const collapsed = sidebarCollapsed;

  useEffect(() => { fetchStats(); }, [fetchStats]);
  useVisibleInterval(fetchStats, 30000);

  useEffect(() => {
    return subscribe((msg) => {
      const m = msg as { type?: string; online_users?: Array<{ username: string; connected_at: string }> };
      if (m.type === "user_online" || m.type === "user_offline") {
        if (m.online_users) setOnlineUsers(m.online_users);
      }
    });
  }, [subscribe, setOnlineUsers]);

  useEffect(() => {
    if (!mobileMenuOpen) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        if (isMobile) setMobileMenuOpen(false);
        else toggleSidebar();
      }
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [mobileMenuOpen, isMobile, toggleSidebar, setMobileMenuOpen]);

  const toggleSection = useCallback((key: string) => {
    setSections((prev) => {
      const next = { ...prev, [key]: !prev[key] };
      saveSectionState(next);
      return next;
    });
  }, []);

  const navContent = (
    <>
      <SidebarLogo
        collapsed={collapsed}
        onToggle={toggleSidebar}
        expandLabel={t("a11y.expand_sidebar")}
        collapseLabel={t("a11y.collapse_sidebar")}
      />
      {!collapsed && (
        <div className="px-2 pt-2">
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder={t("common.search") + "..."}
            aria-label={t("common.search")}
            className="w-full h-8 px-2.5 text-(--fs-xs-sm) bg-secondary/50 border border-border rounded-xl placeholder:text-muted-foreground/70 focus:bg-card focus:border-border transition-colors"
          />
        </div>
      )}
      <div className={`flex-1 overflow-hidden ${collapsed ? 'p-1' : 'p-2'}`}>
        <ScrollArea className="h-full">
          <SidebarNav
            collapsed={collapsed}
            sections={sections}
            toggleSection={toggleSection}
            pathname={pathname}
            stats={stats}
            t={t}
            searchQuery={searchQuery}
          />
        </ScrollArea>
      </div>
      <SidebarFooter
        collapsed={collapsed}
        connected={connected}
        reconnectFailed={reconnectFailed}
        onlineUsers={onlineUsers}
        currentUsername={currentUsername}
        t={t}
      />
    </>
  );

  // Mobile: use Sheet
  if (isMobile) {
    return (
      <Sheet open={mobileMenuOpen} onOpenChange={setMobileMenuOpen}>
        <SheetContent side="left" showCloseButton={false}
          className="w-48 p-0 bg-sidebar border-r border-border">
          <SheetTitle className="sr-only">{t("a11y.navigation")}</SheetTitle>
          <div className="flex flex-col h-full">
            {navContent}
          </div>
        </SheetContent>
      </Sheet>
    );
  }

  // Desktop: fixed aside
  return (
    <aside className={`flex flex-col overflow-hidden transition-all duration-200 ease-in-out h-screen
      bg-sidebar border-r border-border fixed left-0 top-0 z-40
      ${collapsed ? 'w-16' : 'w-48'}`}
    >
      {navContent}
    </aside>
  );
}
