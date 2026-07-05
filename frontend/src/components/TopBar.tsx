"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useWebSocket } from "@/lib/useWebSocket";
import { useI18n } from "@/lib/i18n";
import { useTheme, type Theme } from "@/lib/theme";
import { useAppStore } from "@/lib/store";
import { apiGet } from "@/lib/api";
import { ShortcutsHelpButton } from "@/components/ShortcutsHelp";
import DropdownMenu, { DropdownItem, DropdownDivider, DropdownHeader } from "@/components/DropdownMenu";

interface Notification {
  id: number;
  type: "info" | "warning" | "error" | "success";
  message: string;
  time: string;
  read: boolean;
}

let notifSeq = 1;

export default function TopBar({ sidebarOffset: propOffset, onMenuToggle }: { sidebarOffset?: number; onMenuToggle?: () => void }) {
  const router = useRouter();
  const { locale, setLocale, t } = useI18n();
  const { theme, setTheme } = useTheme();
  const storeSidebarWidth = useAppStore((s) => s.getSidebarWidth());
  const sidebarOffset = propOffset ?? storeSidebarWidth;
  const [searchQuery, setSearchQuery] = useState("");
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [userInfo, setUserInfo] = useState({ name: "admin", role: "Admin", avatar: "A" });

  const pushNotification = useCallback((type: Notification["type"], message: string) => {
    const id = notifSeq++;
    setNotifications((prev) => [
      { id, type, message, time: new Date().toLocaleTimeString(), read: false },
      ...prev.slice(0, 49),
    ]);
  }, []);

  const handleWSMessage = useCallback((msg: { type: string; [key: string]: unknown }) => {
    if (msg.type === "agent_online") pushNotification("success", `Agent online: ${String(msg.hostname || msg.agent_id || "").slice(0, 32)}`);
    else if (msg.type === "agent_offline") pushNotification("warning", `Agent offline: ${String(msg.hostname || msg.agent_id || "").slice(0, 32)}`);
    else if (msg.type === "task_update") {
      const status = String(msg.status || "");
      if (status === "completed") pushNotification("success", `Task done [${String(msg.task_type || "")}]: ${String(msg.command || "").slice(0, 40)}`);
      else if (status === "failed") pushNotification("error", `Task failed [${String(msg.task_type || "")}]: ${String(msg.command || "").slice(0, 40)}`);
    } else if (msg.type === "credential_found") pushNotification("success", String(msg.description || "New credential found"));
    else if (msg.type === "system_alert") pushNotification("warning", String(msg.message || msg.title || "System alert"));
    else if (msg.type === "update_available") pushNotification("info", `Update available: ${String(msg.latest || "")}`);
  }, [pushNotification]);

  const { connected } = useWebSocket(handleWSMessage);

  useEffect(() => {
    apiGet<{ CurrentUsername?: string; CurrentUserRole?: string }>("/settings")
      .then((d) => {
        const name = d.CurrentUsername || "admin";
        const role = d.CurrentUserRole || "Admin";
        setUserInfo({ name, role, avatar: name.charAt(0).toUpperCase() });
      })
      .catch((e) => console.error("TopBar: failed to fetch user info", e));
  }, []);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.key === "k") {
        e.preventDefault();
        const input = document.getElementById("global-search") as HTMLInputElement;
        input?.focus();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const q = searchQuery.trim();
    if (q) router.push(`/search?q=${encodeURIComponent(q)}`);
  };

  const changeLang = (lang: "en" | "zh" | "ja" | "ko" | "ar") => setLocale(lang);

  const unreadCount = notifications.filter((n) => !n.read).length;

  const markAllRead = () => setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));

  const themeOptions: { id: Theme; icon: string; labelKey: string }[] = [
    { id: "light", icon: "fa-sun", labelKey: "topbar.theme_light" },
    { id: "dark", icon: "fa-moon", labelKey: "topbar.theme_dark" },
    { id: "system", icon: "fa-desktop", labelKey: "topbar.theme_system" },
  ];

  const langLabels: Record<string, { flag: string; name: string }> = {
    en: { flag: "🇺🇸", name: "English" },
    zh: { flag: "🇨🇳", name: "中文" },
    ja: { flag: "🇯🇵", name: "日本語" },
    ko: { flag: "🇰🇷", name: "한국어" },
    ar: { flag: "🇸🇦", name: "العربية" },
  };

  return (
    <header className="h-12 bg-[var(--card-bg)] border-b border-[var(--border)] flex items-center justify-between px-3 fixed top-0 right-0 z-30 backdrop-blur-sm bg-opacity-95 transition-[left] duration-200 ease-in-out"
      style={{ left: sidebarOffset }}>
      {/* Left: Menu toggle + Search */}
      <div className="flex items-center gap-3 flex-1 min-w-0">
        <button onClick={onMenuToggle}
          className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-[var(--card-bg-secondary)] text-[var(--text-secondary)] transition-colors lg:hidden">
          <i className="fa-solid fa-bars text-sm"></i>
        </button>
        <form onSubmit={handleSearch} className="relative flex-1 max-w-sm">
          <i className="fa-solid fa-search absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--text-tertiary)] text-xs"></i>
          <input id="global-search" type="text" value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder={t("topbar.search_placeholder")}
            className="w-full bg-[var(--background)] border border-[var(--border)] rounded-lg pl-8 pr-3 h-8 text-xs focus:outline-none focus:border-indigo-500 transition-colors text-[var(--text-primary)] placeholder:text-[var(--text-tertiary)]" />
        </form>
      </div>

      {/* Right: Actions */}
      <div className="flex items-center gap-1">
        {/* WS Status */}
        <div className="flex items-center gap-1.5 px-2 py-1 mr-2" title={connected ? "Real-time connected" : "Disconnected"}>
          <span className={`w-2 h-2 rounded-full ${connected ? "bg-emerald-500 animate-pulse" : "bg-red-500"}`}></span>
          <span className="text-[10px] text-[var(--text-tertiary)] hidden lg:inline">{connected ? "Live" : "Offline"}</span>
        </div>

        <ShortcutsHelpButton />

        {/* Theme */}
        <DropdownMenu trigger={
          <button className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-[var(--card-bg-secondary)] transition-colors text-[var(--text-secondary)]" title={t("topbar.theme")}>
            <i className={`fa-solid ${theme === "dark" ? "fa-moon" : theme === "light" ? "fa-sun" : "fa-desktop"} text-xs`}></i>
          </button>
        }>
          {themeOptions.map((opt) => (
            <DropdownItem key={opt.id} active={theme === opt.id} icon={`fa-solid ${opt.icon}`} onClick={() => setTheme(opt.id)}>
              {t(opt.labelKey)}
            </DropdownItem>
          ))}
        </DropdownMenu>

        {/* Language */}
        <DropdownMenu trigger={
          <button className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-[var(--card-bg-secondary)] transition-colors" title={t("nav.translations")}>
            <span className="text-sm">{langLabels[locale]?.flag || "🌐"}</span>
          </button>
        }>
          {Object.entries(langLabels).map(([code, { flag, name }]) => (
            <DropdownItem key={code} active={locale === code} onClick={() => changeLang(code as "en" | "zh" | "ja" | "ko" | "ar")}>
              <span>{flag}</span> <span>{name}</span>
            </DropdownItem>
          ))}
        </DropdownMenu>

        {/* Notifications */}
        <DropdownMenu trigger={
          <button className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-[var(--card-bg-secondary)] transition-colors relative" title={t("topbar.notifications")}>
            <i className="fa-solid fa-bell text-[var(--text-secondary)] text-sm"></i>
            {unreadCount > 0 && (
              <span className="absolute -top-0.5 -right-0.5 w-4 h-4 bg-red-500 text-white text-[9px] font-bold rounded-full flex items-center justify-center">{unreadCount}</span>
            )}
          </button>
        }>
          <DropdownHeader>
            <div className="flex items-center justify-between">
              <span>{t("topbar.notifications")}</span>
              {unreadCount > 0 && (
                <button onClick={markAllRead} className="text-[10px] text-indigo-600 hover:text-indigo-700">{t("topbar.mark_all_read")}</button>
              )}
            </div>
          </DropdownHeader>
          <div className="max-h-64 overflow-y-auto">
            {notifications.length === 0 ? (
              <div className="p-6 text-center text-[var(--text-tertiary)] text-sm">
                <i className="fa-solid fa-bell-slash text-xl mb-2 block"></i>
                {t("topbar.no_notifications")}
              </div>
            ) : (
              notifications.map((n) => (
                <div key={n.id} className={`px-4 py-3 border-b border-[var(--border)] last:border-0 ${!n.read ? "bg-indigo-50/50 dark:bg-indigo-900/10" : ""}`}>
                  <div className="flex items-start gap-2">
                    <span className={`w-2 h-2 rounded-full mt-1.5 shrink-0 ${n.type === "success" ? "bg-emerald-500" : n.type === "warning" ? "bg-amber-500" : n.type === "error" ? "bg-red-500" : "bg-blue-500"}`}></span>
                    <div className="flex-1 min-w-0">
                      <p className="text-xs text-[var(--text-primary)] truncate">{n.message}</p>
                      <p className="text-[10px] text-[var(--text-tertiary)] mt-0.5">{n.time}</p>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
        </DropdownMenu>

        {/* User */}
        <DropdownMenu trigger={
          <button className="flex items-center gap-2 px-2 py-1.5 rounded-lg hover:bg-[var(--card-bg-secondary)] transition-colors">
            <div className="w-8 h-8 bg-indigo-600 rounded-lg flex items-center justify-center text-white text-xs font-bold">{userInfo.avatar}</div>
            <div className="hidden md:block text-left">
              <div className="text-xs font-medium text-[var(--text-primary)]">{userInfo.name}</div>
              <div className="text-[10px] text-[var(--text-tertiary)]">{userInfo.role}</div>
            </div>
            <i className="fa-solid fa-chevron-down text-[10px] text-[var(--text-tertiary)] hidden md:block"></i>
          </button>
        }>
          <DropdownHeader>
            <div>{userInfo.name}</div>
            <div className="text-[10px] text-[var(--text-tertiary)]">Role: {userInfo.role}</div>
          </DropdownHeader>
          <DropdownItem icon="fa-solid fa-gear" onClick={() => router.push("/settings")}>{t("topbar.settings")}</DropdownItem>
          <DropdownItem icon="fa-solid fa-shield" onClick={() => router.push("/audit")}>{t("topbar.audit_log")}</DropdownItem>
          <DropdownDivider />
          <DropdownItem icon="fa-solid fa-right-from-bracket" danger
            onClick={() => { fetch("/api/go?p=/logout", { method: "POST", credentials: "include" }).finally(() => router.push("/login")); }}>
            {t("topbar.logout")}
          </DropdownItem>
        </DropdownMenu>
      </div>
    </header>
  );
}
