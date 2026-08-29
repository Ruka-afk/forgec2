"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useWebSocket } from "@/lib/useWebSocket";
import { useEffect, useState, useCallback, memo } from "react";
import { useShallow } from "zustand/shallow";
import { useI18n } from "@/lib/i18n";
import { useAppStore, initStatsWSListener } from "@/lib/store";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { logger } from "@/lib/logger";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { POLL } from "@/lib/polling";
import type { DashboardStats } from "@/types/agent";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { StatusDot } from "@/components/ui/status-dot";
import { ConnectionDot } from "@/components/ui/connection-dot";
import { Shield, ChevronDown, LayoutGrid, Settings } from "lucide-react";
import { sidebarNavSections, filterNavByPermissions } from "@/lib/navigation";
import type { PermissionKey } from "@/lib/permission-keys";
import {
  defaultSidebarSections as defaultSections,
  mergeSidebarSections,
  SIDEBAR_SECTIONS_KEY,
  SIDEBAR_SECTIONS_LEGACY_KEY,
} from "@/lib/sidebar-sections";

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
        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-primary to-primary/75 shadow-lg shadow-primary/25 ring-1 ring-primary/20">
          <Shield className="size-4 text-primary-foreground" />
        </div>
        {!collapsed && (
          <div className="text-left leading-tight">
            <span className="font-bold text-xs tracking-tight text-foreground font-mono">Forge</span>
            <span className="font-bold text-xs tracking-tight text-primary font-mono">C2</span>
            <div className="mono-eyebrow text-muted-foreground/100">net · ops</div>
          </div>
        )}
      </Button>
    </div>
  );
});

const SidebarNav = memo(function SidebarNav({ collapsed, sections, toggleSection, pathname, stats, t, searchQuery, permissions }: {
  collapsed: boolean;
  sections: Record<string, boolean>;
  toggleSection: (key: string) => void;
  pathname: string;
  stats: DashboardStats | null;
  t: (key: string, params?: Record<string, string | number>) => string;
  searchQuery: string;
  permissions: readonly PermissionKey[] | null | undefined;
}) {
  function isActive(href: string) {
    if (href === "/dashboard") return pathname === "/dashboard";
    return pathname.startsWith(href);
  }

  const searching = searchQuery.trim().length > 0;
  const query = searching ? searchQuery.toLowerCase() : "";

  const filteredSections = searching
    ? sidebarNavSections().map((section) => ({
        ...section,
        items: filterNavByPermissions(section.items, permissions).filter((item) =>
          t(item.labelKey).toLowerCase().includes(query)
        ),
      })).filter((section) => section.items.length > 0)
    : sidebarNavSections().map((section) => ({
        ...section,
        items: filterNavByPermissions(section.items, permissions),
      })).filter((section) => section.items.length > 0);

  return (
    <nav className={collapsed ? 'flex flex-col items-center gap-1' : 'space-y-0 text-sm'}>
      {filteredSections.map((section, idx) => (
        <div key={section.titleKey} className={collapsed ? 'w-full' : ''}>
          {!collapsed && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => { if (!section.pinned) toggleSection(section.titleKey); }}
              disabled={section.pinned}
              className={`w-full flex items-center gap-x-1 px-2 pt-1 pb-0.5 cursor-pointer select-none transition-colors hover:text-foreground justify-start disabled:opacity-100 disabled:cursor-default ${idx > 0 ? 'mt-2.5 border-t border-border/40 pt-2.5' : ''}`}
            >
              {!section.pinned && (
                <ChevronDown className={`size-3 text-muted-foreground transition-transform duration-200 ${(searching || sections[section.titleKey]) ? '' : '-rotate-90'}`} />
              )}
              <span className="mono-eyebrow text-muted-foreground/85">{t("section." + section.titleKey)}</span>
              {section.titleKey === "lab" && (
                <Badge variant="secondary" className="ml-auto px-1.5 py-px text-(--fs-micro) leading-none rounded bg-warning/15 text-warning-foreground font-medium">
                  {t("section.lab_badge")}
                </Badge>
              )}
            </Button>
          )}
          {(searching || section.pinned || sections[section.titleKey]) && section.items.map((item) => {
            const Icon = item.icon;
            const linkEl = (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-x-2.5 rounded-lg transition-colors duration-150 ${collapsed ? 'group relative' : ''}
                  ${collapsed ? 'justify-center px-0 py-2 mx-auto size-10' : 'px-2 py-1.5'}
                  ${isActive(item.href)
                    ? 'nav-item-active bg-primary/12 font-medium text-primary'
                    : 'text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground'}`}
              >
                <span className="relative inline-flex">
                  <Icon className={collapsed ? 'size-5' : 'size-4'} />
                  {collapsed && item.badge === "agents" && stats != null && (stats.online_agents ?? 0) > 0 && (
                    <StatusDot tone="success" size="xs" className="absolute -top-1 -right-2" />
                  )}
                  {collapsed && item.badge === "listeners" && stats != null && (stats.total_listeners ?? 0) > 0 && (
                    <StatusDot tone="success" size="xs" className="absolute -top-1 -right-2" />
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

const SidebarFooter = memo(function SidebarFooter({ collapsed, connected, reconnectFailed, onlineUsers, currentUsername, t, onOpenTools }: {
  collapsed: boolean;
  connected: boolean;
  reconnectFailed: boolean;
  onlineUsers: Array<{ username: string }>;
  currentUsername: string;
  t: (key: string, params?: Record<string, string | number>) => string;
  onOpenTools: () => void;
}) {
  return (
    <div className={`border-t border-border/60 ${collapsed ? 'px-2 py-2' : 'px-3 py-2.5'} space-y-2`}>
      {!collapsed && connected && (
        <div className="space-y-1">
          <div className="mono-eyebrow text-muted-foreground/100">{t("sidebar.online_operators")}</div>
          {currentUsername && (
              <div className="flex items-center gap-x-2 text-(--fs-xs-sm) text-foreground font-medium">
                <StatusDot tone="success" size="xs" pulse />
                <span className="truncate mono-cell">{t("sidebar.current_operator", { username: currentUsername })}</span>
              </div>
            )}
            {onlineUsers.filter((u) => u.username !== currentUsername).map((u) => (
              <div key={u.username} className="flex items-center gap-x-2 text-(--fs-xs-sm) text-muted-foreground">
                <StatusDot tone="success" size="xs" />
                <span className="truncate">{u.username}</span>
              </div>
            ))}
        </div>
      )}
      <div className={collapsed ? "grid gap-1" : "grid grid-cols-2 gap-1.5"}>
        <Button
          variant="ghost"
          size={collapsed ? "icon" : "sm"}
          onClick={onOpenTools}
          aria-label={t("sidebar.more_tools")}
          className={collapsed ? "mx-auto" : "justify-start"}
        >
          <LayoutGrid className="size-4" />
          {!collapsed && <span>{t("sidebar.more_tools")}</span>}
        </Button>
        <Button
          variant="ghost"
          size={collapsed ? "icon" : "sm"}
          render={<Link href="/settings" />}
          aria-label={t("nav.settings")}
          className={collapsed ? "mx-auto" : "justify-start"}
        >
          <Settings className="size-4" />
          {!collapsed && <span>{t("nav.settings")}</span>}
        </Button>
      </div>
      <div role="status" aria-live="polite" className="flex items-center gap-x-2 rounded-md bg-secondary/50 dark:bg-secondary/30 border border-border/40 px-2 py-1.5">
        <ConnectionDot connected={connected} reconnectFailed={reconnectFailed} />
        <span className="mono-cell text-(--fs-micro-sm) text-muted-foreground/80">
          {connected ? t("common.live") : t("common.disconnected")}
        </span>
        {!connected && <span className={`ml-auto text-(--fs-micro) font-medium ${reconnectFailed ? "text-destructive" : "text-warning"}`}>{reconnectFailed ? t("sidebar.offline") : t("sidebar.reconnecting")}</span>}
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
  const setCommandPaletteOpen = useAppStore((s) => s.setCommandPaletteOpen);
  const onlineUsers = useAppStore(useShallow((s) => s.onlineUsers));
  const setOnlineUsers = useAppStore((s) => s.setOnlineUsers);
  const currentUsername = useAppStore((s) => s.currentUsername);
  const setCurrentUsername = useAppStore((s) => s.setCurrentUsername);
  const setCurrentUserRole = useAppStore((s) => s.setCurrentUserRole);
  const setCurrentPermissions = useAppStore((s) => s.setCurrentPermissions);
  const permissions = useAppStore((s) => s.currentPermissions);

  useEffect(() => { Promise.resolve().then(() => setSections(getSavedSections())); }, []);

  useEffect(() => {
    if (currentUsername) return;
    api.get<{ data?: { username?: string; role?: string; permissions?: string[] } }>(paths.auth.me).then((d) => {
      const me = d?.data;
      if (me?.username) setCurrentUsername(me.username);
      if (me?.role) setCurrentUserRole(me.role);
      if (Array.isArray(me?.permissions)) setCurrentPermissions(me.permissions as PermissionKey[]);
    }).catch((e) => { if (process.env.NODE_ENV === "development") logger.error("fetch current user failed", e); });
  }, [currentUsername, setCurrentUsername, setCurrentUserRole, setCurrentPermissions]);

  const collapsed = sidebarCollapsed;

  useEffect(() => { fetchStats(); }, [fetchStats]);
  useEffect(() => { initStatsWSListener(); }, []);
  useVisibleInterval(fetchStats, POLL.stats);

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
          <Input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder={t("common.search") + "..."}
            aria-label={t("common.search")}
            className="h-8 bg-secondary/50 text-(--fs-xs-sm) placeholder:text-muted-foreground/100"
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
            permissions={permissions}
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
        onOpenTools={() => setCommandPaletteOpen(true)}
      />
    </>
  );

  // Mobile: use Sheet
  if (isMobile) {
    return (
      <Sheet open={mobileMenuOpen} onOpenChange={setMobileMenuOpen}>
        <SheetContent side="left" showCloseButton={false}
          className="w-72 p-0 bg-sidebar border-r border-border">
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
      ${collapsed ? 'w-[var(--shell-sidebar-collapsed)]' : 'w-[var(--shell-sidebar-expanded)]'}`}
    >
      {navContent}
    </aside>
  );
}
