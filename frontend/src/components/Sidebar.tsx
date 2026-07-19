"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useWebSocket } from "@/lib/useWebSocket";
import { useEffect, useState, useCallback } from "react";
import { useI18n } from "@/lib/i18n";
import { useAppStore } from "@/lib/store";
import { api } from "@/lib/api";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import type { DashboardStats } from "@/types/agent";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import {
  Activity, Bot, Shield, Fish, Zap, Bug, Tags, Layers, Wand2, Clock,
  FolderTree, Bell, MessageSquare, GitBranch, Link as LinkIcon, Boxes,
  Radio, Hammer, Server, Cloud, PenTool, Box, Wrench, Code, Key,
  Route, IdCard, Archive, Images, SatelliteDish, ArrowLeftRight,
  FileCode, FileText, Globe, Puzzle, Network, Crosshair, ClipboardList,
  Plug, Users, Settings, Book, Plus, RefreshCw, ChevronDown, Search,
  type LucideIcon,
} from "lucide-react";

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

const SECTION_KEYS = ["operations", "build-deploy", "post-exploitation", "intel-analysis", "system"];

const navSections: NavSection[] = [
  {
    titleKey: "operations",
    items: [
      { href: "/dashboard", labelKey: "nav.dashboard", icon: Activity },
      { href: "/timeline", labelKey: "nav.timeline", icon: Clock },
      { href: "/automation", labelKey: "nav.automation", icon: Bot },
      { href: "/opsec", labelKey: "nav.opsec", icon: Shield },
      { href: "/phishing", labelKey: "nav.phishing", icon: Fish },
      { href: "/circuit-breaker", labelKey: "nav.circuit_breaker", icon: Zap },
      { href: "/agents", labelKey: "nav.beacons", icon: Bug, badge: "agents" },
      { href: "/tags", labelKey: "nav.tags", icon: Tags },
      { href: "/groups", labelKey: "nav.groups", icon: Layers },
      { href: "/autotag", labelKey: "nav.autotag", icon: Wand2 },
      { href: "/scheduler", labelKey: "nav.scheduler", icon: Clock },
      { href: "/files", labelKey: "nav.files", icon: FolderTree },
      { href: "/notifications", labelKey: "nav.notifications", icon: Bell },
      { href: "/chat", labelKey: "nav.chat", icon: MessageSquare },
      { href: "/topology", labelKey: "nav.topology", icon: GitBranch },
      { href: "/chain", labelKey: "nav.chain", icon: LinkIcon },
      { href: "/search", labelKey: "nav.search", icon: Search },
    ],
  },
  {
    titleKey: "build-deploy",
    items: [
      { href: "/generate", labelKey: "nav.generate", icon: Boxes },
      { href: "/listeners", labelKey: "nav.listeners", icon: Radio, badge: "listeners" },
      { href: "/builds", labelKey: "nav.builds", icon: Hammer },
      { href: "/infrastructure", labelKey: "nav.infrastructure", icon: Server },
      { href: "/domain-fronting", labelKey: "nav.domain_fronting", icon: Cloud },
      { href: "/profiles", labelKey: "nav.profiles", icon: PenTool },
      { href: "/packer", labelKey: "nav.packer", icon: Box },
      { href: "/stager", labelKey: "nav.stager", icon: GitBranch },
      { href: "/dns", labelKey: "nav.dns", icon: Network },
    ],
  },
  {
    titleKey: "post-exploitation",
    items: [
      { href: "/tasks", labelKey: "nav.tasks", icon: Clock },
      { href: "/workflows", labelKey: "nav.workflows", icon: GitBranch },
      { href: "/toolkit", labelKey: "nav.toolkit", icon: Wrench },
      { href: "/scripting", labelKey: "nav.scripting", icon: Code },
      { href: "/credentials", labelKey: "nav.credentials", icon: Key },
      { href: "/pivoting", labelKey: "nav.pivoting", icon: Route },
      { href: "/tokens", labelKey: "nav.token_store", icon: IdCard },
      { href: "/loot", labelKey: "nav.loot", icon: Archive },
      { href: "/screenshots", labelKey: "nav.screenshots", icon: Images },
      { href: "/scanner", labelKey: "nav.scanner", icon: SatelliteDish },
      { href: "/privesc", labelKey: "nav.privesc", icon: Shield },
      { href: "/lateral", labelKey: "nav.lateral", icon: ArrowLeftRight },
      { href: "/bof", labelKey: "nav.bof", icon: FileCode },
      { href: "/command_templates", labelKey: "nav.templates", icon: FileText },
      { href: "/chrome", labelKey: "nav.chrome_c2", icon: Globe },
      { href: "/cloud", labelKey: "nav.cloud", icon: Cloud },
      { href: "/ntlm", labelKey: "nav.ntlm", icon: Zap },
      { href: "/container", labelKey: "nav.container", icon: Box },
      { href: "/plugins", labelKey: "nav.plugins", icon: Puzzle },
    ],
  },
  {
    titleKey: "intel-analysis",
    items: [
      { href: "/audit", labelKey: "nav.audit", icon: Shield },
      { href: "/traffic", labelKey: "nav.traffic", icon: Network },
      { href: "/campaign", labelKey: "nav.campaign", icon: Crosshair },
      { href: "/ai", labelKey: "nav.ai", icon: Bot },
      { href: "/report", labelKey: "nav.report", icon: ClipboardList },
      { href: "/attack", labelKey: "nav.attack", icon: Shield },
      { href: "/integrations", labelKey: "nav.integrations", icon: Plug },
      { href: "/bloodhound", labelKey: "nav.bloodhound", icon: Network },
    ],
  },
  {
    titleKey: "system",
    items: [
      { href: "/users", labelKey: "nav.users", icon: Users },
      { href: "/roles", labelKey: "nav.roles", icon: Shield },
      { href: "/settings", labelKey: "nav.settings", icon: Settings },
      { href: "/docs", labelKey: "nav.docs", icon: Book },
    ],
  },
];

const quickActions = [
  { href: "/generate", key: "nav.generate", icon: Plus },
  { href: "/listeners", key: "nav.listeners", icon: Plug },
  { href: "/agents", key: "common.refresh", icon: RefreshCw },
];

const defaultSections: Record<string, boolean> = Object.fromEntries(SECTION_KEYS.map(k => [k, true]));

let _savedSections: Record<string, boolean> | null = null;
function getSavedSections(): Record<string, boolean> {
  if (_savedSections) return _savedSections;
  try {
    const raw = localStorage.getItem('forgec2_sidebar_sections');
    if (!raw) return defaultSections;
    const saved = JSON.parse(raw);
    if (saved && typeof saved === 'object' && !Array.isArray(saved)) { _savedSections = saved; return saved; }
  } catch { /* ignore */ }
  return defaultSections;
}

function saveSectionState(state: Record<string, boolean>) {
  _savedSections = state;
  try { localStorage.setItem('forgec2_sidebar_sections', JSON.stringify(state)); }
  catch { /* ignore */ }
}

function SidebarLogo({ collapsed, onToggle }: { collapsed: boolean; onToggle: () => void }) {
  return (
    <div className={`border-b border-border flex items-center ${collapsed ? 'justify-center px-0 py-3' : 'px-4 py-3'}`}>
      <Button variant="ghost" onClick={onToggle}
        className={`flex items-center gap-x-2.5 hover:opacity-80 transition-all duration-200 ${collapsed ? 'justify-center w-full' : ''}`}
        aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}>
        <div className="w-8 h-8 bg-gradient-to-br from-indigo-500 to-indigo-700 rounded-xl flex items-center justify-center shadow-lg shadow-indigo-500/25 shrink-0">
          <Shield className="w-4 h-4 text-white" />
        </div>
        {!collapsed && (
          <div className="text-left">
            <span className="font-bold text-base tracking-tight text-foreground">Forge</span>
            <span className="font-bold text-base tracking-tight text-primary">C2</span>
          </div>
        )}
      </Button>
    </div>
  );
}

function SidebarNav({ collapsed, sections, toggleSection, pathname, stats, t, searchQuery }: {
  collapsed: boolean;
  sections: Record<string, boolean>;
  toggleSection: (key: string) => void;
  pathname: string;
  stats: DashboardStats | null;
  t: (key: string) => string;
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
    <nav className={collapsed ? 'flex flex-col items-center gap-1' : 'space-y-0 text-[13px]'}>
      {filteredSections.map((section) => (
        <div key={section.titleKey} className={collapsed ? 'w-full' : ''}>
          {!collapsed && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => toggleSection(section.titleKey)}
              className="w-full flex items-center gap-x-1 px-3 pt-2 pb-0.5 cursor-pointer select-none transition-colors hover:text-foreground justify-start"
            >
              <ChevronDown className={`w-3 h-3 text-muted-foreground transition-transform duration-200 ${(searching || sections[section.titleKey]) ? '' : '-rotate-90'}`} />
              <span className="text-[10px] uppercase tracking-wider text-muted-foreground/70 font-semibold">
                {t("section." + section.titleKey)}
              </span>
            </Button>
          )}
          {(searching || sections[section.titleKey]) && section.items.map((item) => {
            const Icon = item.icon;
            const linkEl = (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-x-2.5 rounded-lg transition-all duration-150 hover:translate-x-0.5 ${collapsed ? 'group relative' : ''}
                  ${collapsed ? 'justify-center px-0 py-2 mx-auto w-10 h-10' : 'px-3 py-1.5'}
                  ${isActive(item.href)
                    ? 'bg-primary/10 text-primary font-medium border-l-2 border-primary shadow-sm'
                    : 'text-muted-foreground hover:bg-secondary/80 hover:text-foreground'}`}
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
                    <span className="truncate flex-1 text-[13px]">{t(item.labelKey)}</span>
                    {item.badge === "agents" && stats != null && (
                      <span className="px-1.5 py-px text-[9px] leading-none rounded bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 font-mono">
                        {stats.online_agents ?? 0}
                      </span>
                    )}
                    {item.badge === "listeners" && stats != null && (
                      <span className="px-1.5 py-px text-[9px] leading-none rounded bg-primary/10 text-primary font-mono">
                        {stats.total_listeners ?? 0}
                      </span>
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
}

function QuickActions({ collapsed, t }: { collapsed: boolean; t: (key: string) => string }) {
  if (collapsed) {
    return (
      <div className="mt-4 flex flex-col items-center gap-1">
        {quickActions.map((action) => {
          const Icon = action.icon;
          return (
            <Tooltip key={action.href}>
              <TooltipTrigger render={
                <Link href={action.href} className="w-9 h-9 flex items-center justify-center rounded-lg text-muted-foreground hover:bg-secondary/80 hover:text-primary transition-colors">
                  <Icon className="w-4 h-4" />
                </Link>
              } />
              <TooltipContent side="right">{t(action.key)}</TooltipContent>
            </Tooltip>
          );
        })}
      </div>
    );
  }
  return (
    <div className="mt-4 px-2">
      <div className="text-[10px] uppercase tracking-widest text-muted-foreground/70 px-3 mb-1">Quick Actions</div>
      <div className="flex flex-wrap gap-1.5">
        {quickActions.map((action) => {
          const Icon = action.icon;
          return (
            <Link
              key={action.href}
              href={action.href}
              className="text-xs px-2.5 py-1.5 bg-secondary/80 hover:bg-primary/10 dark:hover:bg-primary/15 rounded-lg flex items-center gap-x-1.5 text-muted-foreground hover:text-primary transition-all duration-150"
            >
              <Icon className="w-3 h-3" />
              <span className="text-[11px]">{t(action.key)}</span>
            </Link>
          );
        })}
      </div>
    </div>
  );
}

function SidebarFooter({ collapsed, connected, onlineUsers, currentUsername, t }: {
  collapsed: boolean;
  connected: boolean;
  onlineUsers: Array<{ username: string }>;
  currentUsername: string;
  t: (key: string) => string;
}) {
  return (
    <div className={`border-t border-border ${collapsed ? 'px-2 py-3' : 'px-4 py-3'} space-y-2`}>
      {!collapsed && connected && (
        <div className="space-y-1">
          <div className="text-[10px] uppercase tracking-wider text-muted-foreground/70 font-semibold">Online Operators</div>
          {currentUsername && (
            <div className="flex items-center gap-x-2 text-[11px] text-foreground font-medium">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0 animate-pulse" />
              <span className="truncate">{currentUsername} (you)</span>
            </div>
          )}
          {onlineUsers.filter((u) => u.username !== currentUsername).map((u) => (
            <div key={u.username} className="flex items-center gap-x-2 text-[11px] text-muted-foreground">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0" />
              <span className="truncate">{u.username}</span>
            </div>
          ))}
        </div>
      )}
      <div className="flex items-center gap-x-2">
        <span className={`w-2 h-2 rounded-full ${connected ? "bg-emerald-500 animate-pulse" : "bg-red-500"}`} />
        <span className="text-[10px] text-muted-foreground/70">
          {connected ? t("common.live") : t("common.disconnected")}
        </span>
        {!connected && <span className="ml-auto text-[9px] text-destructive font-medium">RECONNECTING</span>}
      </div>
    </div>
  );
}

export default function Sidebar() {
  const pathname = usePathname();
  const { connected, lastMessage } = useWebSocket();
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
  const onlineUsers = useAppStore((s) => s.onlineUsers);
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
    if (!lastMessage) return;
    const msg = lastMessage as { type?: string; online_users?: Array<{ username: string; connected_at: string }> };
    if (msg.type === "user_online" || msg.type === "user_offline") {
      if (msg.online_users) setOnlineUsers(msg.online_users);
    }
  }, [lastMessage, setOnlineUsers]);

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
      <SidebarLogo collapsed={collapsed} onToggle={toggleSidebar} />
      {!collapsed && (
        <div className="px-3 pt-2">
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder={t("common.search") + "..."}
            className="w-full h-8 px-3 text-xs bg-secondary/50 border border-border rounded-lg placeholder:text-muted-foreground/70 focus:bg-card focus:border-border transition-colors"
          />
        </div>
      )}
      <div className={`flex-1 overflow-hidden ${collapsed ? 'p-2' : 'p-3'}`}>
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
          <QuickActions collapsed={collapsed} t={t} />
        </ScrollArea>
      </div>
      <SidebarFooter
        collapsed={collapsed}
        connected={connected}
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
          className="w-56 p-0 bg-sidebar border-r border-border">
          <SheetTitle className="sr-only">Navigation</SheetTitle>
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
      ${collapsed ? 'w-16' : 'w-56'}`}
    >
      {navContent}
    </aside>
  );
}
