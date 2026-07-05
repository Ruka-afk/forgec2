"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useWebSocket } from "@/lib/useWebSocket";
import { useEffect, useState, useCallback } from "react";
import { useI18n } from "@/lib/i18n";
import { useAppStore } from "@/lib/store";

interface NavItem {
  href: string;
  labelKey: string;
  icon: string;
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
      { href: "/dashboard", labelKey: "nav.dashboard", icon: "fa-solid fa-chart-line" },
      { href: "/search", labelKey: "nav.search", icon: "fa-solid fa-magnifying-glass" },
      { href: "/timeline", labelKey: "nav.timeline", icon: "fa-solid fa-timeline" },
      { href: "/automation", labelKey: "nav.automation", icon: "fa-solid fa-robot" },
      { href: "/opsec", labelKey: "nav.opsec", icon: "fa-solid fa-shield-halved" },
      { href: "/circuit-breaker", labelKey: "nav.circuit_breaker", icon: "fa-solid fa-bolt" },
      { href: "/agents", labelKey: "nav.beacons", icon: "fa-solid fa-bug", badge: "agents" },
      { href: "/topology", labelKey: "nav.topology", icon: "fa-solid fa-diagram-project" },
    ],
  },
  {
    titleKey: "build-deploy",
    items: [
      { href: "/generate", labelKey: "nav.generate", icon: "fa-solid fa-cubes" },
      { href: "/listeners", labelKey: "nav.listeners", icon: "fa-solid fa-tower-broadcast", badge: "listeners" },
      { href: "/builds", labelKey: "nav.builds", icon: "fa-solid fa-hammer" },
      { href: "/infrastructure", labelKey: "nav.infrastructure", icon: "fa-solid fa-server" },
    ],
  },
  {
    titleKey: "post-exploitation",
    items: [
      { href: "/tasks", labelKey: "nav.tasks", icon: "fa-solid fa-history" },
      { href: "/toolkit", labelKey: "nav.toolkit", icon: "fa-solid fa-screwdriver-wrench" },
      { href: "/scripting", labelKey: "nav.scripting", icon: "fa-solid fa-code" },
      { href: "/credentials", labelKey: "nav.credentials", icon: "fa-solid fa-key" },
      { href: "/pivoting", labelKey: "nav.pivoting", icon: "fa-solid fa-arrows-turn-to-dots" },
      { href: "/tokens", labelKey: "nav.token_store", icon: "fa-solid fa-id-badge" },
      { href: "/loot", labelKey: "nav.loot", icon: "fa-solid fa-box-archive" },
      { href: "/scanner", labelKey: "nav.scanner", icon: "fa-solid fa-satellite-dish" },
      { href: "/privesc", labelKey: "nav.privesc", icon: "fa-solid fa-shield-halved" },
      { href: "/lateral", labelKey: "nav.lateral", icon: "fa-solid fa-arrows-left-right" },
      { href: "/bof", labelKey: "nav.bof", icon: "fa-solid fa-file-code" },
      { href: "/command_templates", labelKey: "nav.templates", icon: "fa-solid fa-file-lines" },
      { href: "/plugins", labelKey: "nav.plugins", icon: "fa-solid fa-puzzle-piece" },
    ],
  },
  {
    titleKey: "intel-analysis",
    items: [
      { href: "/audit", labelKey: "nav.audit", icon: "fa-solid fa-shield" },
      { href: "/traffic", labelKey: "nav.traffic", icon: "fa-solid fa-network-wired" },
      { href: "/ai", labelKey: "nav.ai", icon: "fa-solid fa-robot" },
      { href: "/report", labelKey: "nav.report", icon: "fa-solid fa-clipboard-list" },
      { href: "/translations", labelKey: "nav.translations", icon: "fa-solid fa-language" },
    ],
  },
  {
    titleKey: "system",
    items: [
      { href: "/users", labelKey: "nav.users", icon: "fa-solid fa-users-gear" },
      { href: "/settings", labelKey: "nav.settings", icon: "fa-solid fa-gear" },
      { href: "/docs", labelKey: "nav.docs", icon: "fa-solid fa-book" },
    ],
  },
];

const quickActions = [
  { href: "/generate", key: "nav.generate", icon: "fa-solid fa-plus" },
  { href: "/listeners", key: "nav.listeners", icon: "fa-solid fa-plug" },
  { href: "/agents", key: "common.refresh", icon: "fa-solid fa-sync" },
];

const defaultSections: Record<string, boolean> = Object.fromEntries(SECTION_KEYS.map(k => [k, true]));

let _savedSections: Record<string, boolean> | null = null;
function getSavedSections(): Record<string, boolean> {
  if (_savedSections) return _savedSections;
  try {
    const saved = JSON.parse(localStorage.getItem('forgec2_sidebar_sections') || '');
    if (saved && typeof saved === 'object') { _savedSections = saved; return saved; }
  } catch { /* ignore */ }
  return defaultSections;
}

function saveSectionState(state: Record<string, boolean>) {
  _savedSections = state;
  try { localStorage.setItem('forgec2_sidebar_sections', JSON.stringify(state)); }
  catch { /* ignore */ }
}

export default function Sidebar() {
  const pathname = usePathname();
  const { connected, lastMessage } = useWebSocket();
  const { t } = useI18n();
  const [sections, setSections] = useState<Record<string, boolean>>(defaultSections);
  const { stats, fetchStats, sidebarCollapsed, isMobile, mobileMenuOpen, toggleSidebar, setMobileMenuOpen, onlineUsers, setOnlineUsers, currentUsername, setCurrentUsername } = useAppStore();

  // Load saved section state from localStorage after mount (hydration-safe)
  useEffect(() => { setSections(getSavedSections()); }, []);

  // Fetch current username
  useEffect(() => {
    if (currentUsername) return;
    import("@/lib/api").then(({ apiGet }) =>
      apiGet<{ CurrentUsername?: string }>("/settings").then((d) => {
        if (d.CurrentUsername) setCurrentUsername(d.CurrentUsername);
      }).catch((e) => console.error("Sidebar: failed to fetch username", e))
    );
  }, [currentUsername, setCurrentUsername]);

  const collapsed = sidebarCollapsed;
  const mobileOpen = isMobile ? mobileMenuOpen : true;

  useEffect(() => {
    fetchStats();
    const interval = setInterval(() => {
      fetchStats();
    }, 30000);
    return () => clearInterval(interval);
  }, [fetchStats]);

  // Listen for user_online / user_offline WebSocket events
  useEffect(() => {
    if (!lastMessage) return;
    const msg = lastMessage as { type?: string; online_users?: Array<{ username: string; connected_at: string }> };
    if (msg.type === "user_online" || msg.type === "user_offline") {
      if (msg.online_users) {
        setOnlineUsers(msg.online_users);
      }
    }
  }, [lastMessage, setOnlineUsers]);

  useEffect(() => {
    if (!mobileOpen) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") { toggleSidebar(); }
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [mobileOpen, toggleSidebar]);

  const toggleSection = useCallback((key: string) => {
    setSections((prev) => {
      const next = { ...prev, [key]: !prev[key] };
      saveSectionState(next);
      return next;
    });
  }, []);

  function isActive(href: string) {
    if (href === "/dashboard") return pathname === "/dashboard";
    return pathname.startsWith(href);
  }

  return (
    <>
      {/* Mobile backdrop */}
      {isMobile && mobileOpen && (
        <div className="fixed inset-0 z-40 bg-black/40" onClick={() => setMobileMenuOpen(false)} />
      )}
      <aside
        className={`flex flex-col overflow-hidden transition-all duration-200 ease-in-out
          bg-[var(--sidebar-bg)] border-r border-[var(--border)]
          ${isMobile ? (mobileOpen ? 'fixed left-0 top-0 z-50 h-screen shadow-2xl' : 'hidden') : 'relative'}
          ${collapsed ? 'w-16' : 'w-56'}`}
        style={{ minHeight: "100vh" }}
      >
        {/* Logo */}
        <div className={`border-b border-[var(--border)] flex items-center ${collapsed ? 'justify-center px-0 py-3' : 'px-4 py-3'}`}>
          <button onClick={toggleSidebar} className={`flex items-center gap-x-2 hover:opacity-80 transition-opacity ${collapsed ? 'justify-center w-full' : ''}`}>
            <div className="w-8 h-8 bg-gradient-to-br from-indigo-500 to-indigo-700 rounded-lg flex items-center justify-center shadow-lg shadow-indigo-500/20 shrink-0">
              <i className="fa-solid fa-shield-halved text-white text-sm"></i>
            </div>
            {!collapsed && (
              <div className="text-left">
                <span className="font-bold text-base tracking-tight text-[var(--text-primary)]">Forge</span>
                <span className="font-bold text-base tracking-tight text-indigo-600 dark:text-indigo-400">C2</span>
              </div>
            )}
          </button>
        </div>

        {/* Scrollable nav area */}
        <div className={`flex-1 overflow-y-auto ${collapsed ? 'p-2' : 'p-3'}`}>
          <nav className={collapsed ? 'flex flex-col items-center gap-1' : 'space-y-0 text-[13px]'}>
            {navSections.map((section) => (
              <div key={section.titleKey} className={collapsed ? 'w-full' : ''}>
                {!collapsed && (
                  <div
                    onClick={() => toggleSection(section.titleKey)}
                    className={`section-header flex items-center gap-x-1 px-3 pt-2 pb-0.5 cursor-pointer
                      ${sections[section.titleKey] ? 'collapsed' : ''}`}
                  >
                    <i className={`fa-solid w-3 text-[9px] transition-transform ${sections[section.titleKey] ? '' : 'rotate-[-90deg]'}`}></i>
                    <span className="text-[10px] uppercase tracking-wider text-[var(--text-tertiary)] font-semibold">
                      {t("section." + section.titleKey)}
                    </span>
                  </div>
                )}
                {(!collapsed && !sections[section.titleKey]) || collapsed
                  ? section.items.map((item) => (
                      <Link
                        key={item.href}
                        href={item.href}
                        title={collapsed ? t(item.labelKey) : undefined}
                        className={`flex items-center gap-x-2.5 rounded-lg transition-all duration-150
                          ${collapsed
                            ? 'justify-center px-0 py-2 mx-auto w-10 h-10'
                            : 'px-3 py-1.5'}
                          ${isActive(item.href)
                            ? 'nav-active'
                            : 'text-[var(--text-secondary)] hover:bg-[var(--card-bg-secondary)] hover:text-[var(--text-primary)]'}`}
                      >
                        <i className={`${item.icon} w-4 text-center ${collapsed ? 'text-sm' : 'text-xs'}`}></i>
                        {!collapsed && (
                          <>
                            <span className="truncate flex-1 text-[13px]">{t(item.labelKey)}</span>
                            {item.badge === "agents" && stats != null && (
                              <span className="px-1.5 py-px text-[9px] leading-none rounded bg-emerald-500/20 text-emerald-600 dark:text-emerald-400 font-mono">
                                {stats.OnlineAgents ?? stats.online_agents ?? 0}
                              </span>
                            )}
                            {item.badge === "listeners" && stats != null && (
                              <span className="px-1.5 py-px text-[9px] leading-none rounded bg-indigo-500/20 text-indigo-600 dark:text-indigo-400 font-mono">
                                {stats.TotalListeners ?? stats.total_listeners ?? 0}
                              </span>
                            )}
                          </>
                        )}
                      </Link>
                    ))
                  : null}
              </div>
            ))}
          </nav>

          {/* Quick Actions */}
          {!collapsed && (
            <div className="mt-4 px-2">
              <div className="text-[10px] uppercase tracking-widest text-[var(--text-tertiary)] px-3 mb-1">Quick Actions</div>
              <div className="flex flex-wrap gap-1.5">
                {quickActions.map((action) => (
                  <Link
                    key={action.href}
                    href={action.href}
                    className="text-xs px-2.5 py-1 bg-[var(--card-bg-secondary)] hover:bg-indigo-100 dark:hover:bg-indigo-900/30 rounded-xl flex items-center gap-x-1 text-[var(--text-secondary)] transition-colors"
                  >
                    <i className={`${action.icon} text-[10px]`}></i>
                    <span className="text-[11px]">{t(action.key)}</span>
                  </Link>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className={`border-t border-[var(--border)] ${collapsed ? 'px-2 py-3' : 'px-4 py-3'} space-y-2`}>
          {/* Online users */}
          {!collapsed && connected && (
            <div className="space-y-1">
              <div className="text-[10px] uppercase tracking-wider text-[var(--text-tertiary)] font-semibold">
                Online Operators
              </div>
              {/* Always show current user */}
              {currentUsername && (
                <div className="flex items-center gap-x-2 text-[11px] text-[var(--text-primary)] font-medium">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0 animate-pulse"></span>
                  <span className="truncate">{currentUsername} (you)</span>
                </div>
              )}
              {/* Other online users */}
              {onlineUsers.filter((u) => u.username !== currentUsername).map((u) => (
                <div key={u.username} className="flex items-center gap-x-2 text-[11px] text-[var(--text-secondary)]">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0"></span>
                  <span className="truncate">{u.username}</span>
                </div>
              ))}
            </div>
          )}
          {/* Connection status */}
          <div className="flex items-center gap-x-2">
            <span className={`w-2 h-2 rounded-full ${connected ? "bg-emerald-500 animate-pulse" : "bg-red-500"}`}></span>
            <span className="text-[10px] text-[var(--text-tertiary)]">
              {connected ? t("common.live") : t("common.disconnected")}
            </span>
            {!connected && (
              <span className="ml-auto text-[9px] text-red-400 font-medium">RECONNECTING</span>
            )}
          </div>
        </div>
      </aside>
    </>
  );
}
