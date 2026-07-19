"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useWebSocket } from "@/lib/useWebSocket";
import { useI18n } from "@/lib/i18n";
import { useTheme, type Theme } from "@/lib/theme";
import { useAppStore } from "@/lib/store";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { ShortcutsHelpButton } from "@/components/ShortcutsHelp";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Menu, Search, X, Moon, Sun, Monitor, Bell, BellOff,
  Settings, Shield, LogOut,
} from "lucide-react";

interface Notification {
  id: number;
  type: "info" | "warning" | "error" | "success";
  message: string;
  time: string;
  read: boolean;
}

let notifSeq = 1;

const THEME_ICONS: Record<Theme, React.ComponentType<{ className?: string }>> = {
  light: Sun,
  dark: Moon,
  system: Monitor,
};

const THEME_OPTIONS: { value: Theme; labelKey: string }[] = [
  { value: "light", labelKey: "topbar.theme_light" },
  { value: "dark", labelKey: "topbar.theme_dark" },
  { value: "system", labelKey: "topbar.theme_system" },
];

const LANG_OPTIONS = [
  { value: "en", flag: "\u{1F1FA}\u{1F1F8}", name: "English" },
  { value: "zh", flag: "\u{1F1E8}\u{1F1F3}", name: "\u4E2D\u6587" },
];

function SearchBox() {
  const [query, setQuery] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);
  const router = useRouter();
  const { t } = useI18n();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const q = query.trim();
    if (q) { router.push(`/search?q=${encodeURIComponent(q)}`); setMobileOpen(false); }
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.key === "k") {
        e.preventDefault();
        document.getElementById("global-search")?.focus();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  return (
    <>
      {/* Desktop search */}
      <form onSubmit={handleSubmit} className="relative flex-1 max-w-sm hidden sm:flex">
        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground/70" />
        <Input id="global-search" type="text" value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("topbar.search_placeholder")}
          className="h-8 pl-8 pr-16 text-xs placeholder:text-muted-foreground/70 bg-secondary/50 border-transparent focus:bg-card focus:border-border" />
        {query && (
          <Button type="button" onClick={() => setQuery("")}
            variant="ghost" size="icon-xs" className="absolute right-8 top-1/2 -translate-y-1/2" aria-label="Clear search">
            <X className="w-3 h-3" />
          </Button>
        )}
        <kbd className="absolute right-2 top-1/2 -translate-y-1/2 text-[10px] text-muted-foreground/70 bg-secondary px-1.5 py-0.5 rounded border border-border">
          {typeof navigator !== "undefined" && /Mac/.test(navigator.platform) ? "\u2318K" : "Ctrl+K"}
        </kbd>
      </form>
      {/* Mobile search toggle */}
      <Button variant="ghost" size="icon" className="sm:hidden" onClick={() => setMobileOpen(!mobileOpen)} aria-label="Search">
        <Search className="w-5 h-5" />
      </Button>
      {mobileOpen && (
        <form onSubmit={handleSubmit} className="sm:hidden absolute top-full left-0 right-0 z-50 p-3 bg-card border-b border-border shadow-lg animate-fade-in">
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground/70" />
            <Input type="text" value={query} autoFocus
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("topbar.search_placeholder")}
              className="h-9 pl-8 pr-8 text-sm bg-secondary/50" />
            {query && (
              <Button type="button" onClick={() => setQuery("")} variant="ghost" size="icon-xs"
                className="absolute right-2 top-1/2 -translate-y-1/2" aria-label="Clear">
                <X className="w-3 h-3" />
              </Button>
            )}
          </div>
        </form>
      )}
    </>
  );
}

function ThemeSelector() {
  const { theme, setTheme } = useTheme();
  const { t } = useI18n();
  const ThemeIcon = THEME_ICONS[theme];

  return (
    <Select value={theme} onValueChange={(v) => setTheme(v as Theme)}>
      <SelectTrigger size="sm" className="w-8 h-8 p-0 justify-center" title={t("topbar.theme")} aria-label={t("topbar.theme")}>
        <SelectValue>
          <ThemeIcon className="w-4 h-4" />
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {THEME_OPTIONS.map((opt) => {
          const Icon = THEME_ICONS[opt.value];
          return (
            <SelectItem key={opt.value} value={opt.value}>
              <Icon className="w-4 h-4" />
              <span>{t(opt.labelKey)}</span>
            </SelectItem>
          );
        })}
      </SelectContent>
    </Select>
  );
}

function LanguageSelector() {
  const { locale, setLocale } = useI18n();
  const currentLang = LANG_OPTIONS.find((l) => l.value === locale) || LANG_OPTIONS[0];

  return (
    <Select value={locale} onValueChange={(v) => setLocale(v as "en" | "zh")}>
      <SelectTrigger size="sm" className="w-8 h-8 p-0 justify-center" title="Language" aria-label="Language">
        <SelectValue>
          <span className="text-sm">{currentLang.flag}</span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {LANG_OPTIONS.map((lang) => (
          <SelectItem key={lang.value} value={lang.value}>
            <span>{lang.flag}</span>
            <span>{lang.name}</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function NotificationDropdown() {
  const { t } = useI18n();
  const [notifications, setNotifications] = useState<Notification[]>([]);

  useEffect(() => {
    api.get("/notifications?page=1&pageSize=20")
      .then((data) => {
        const list = (data.notifications || data.data || []) as Array<Record<string, unknown>>;
        const mapped: Notification[] = list.slice(0, 20).map((n) => ({
          id: Number(n.id) || 0,
          type: (["info", "warning", "error", "success"].includes(String(n.severity || n.type || "info"))
            ? String(n.severity || n.type || "info")
            : "info") as Notification["type"],
          message: String(n.message || n.title || ""),
          time: String(n.created_at || new Date().toLocaleTimeString()),
          read: Boolean(n.read),
        }));
        setNotifications(mapped);
      })
      .catch(() => { /* silent */ });
  }, []);

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

  useWebSocket(handleWSMessage);

  const unreadCount = notifications.filter((n) => !n.read).length;
  const markAllRead = () => setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));

  const typeColors: Record<string, string> = {
    success: "bg-emerald-500",
    warning: "bg-amber-500",
    error: "bg-red-500",
    info: "bg-blue-500",
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={
        <Button variant="ghost" size="icon" className="relative" title={t("topbar.notifications")} aria-label={t("topbar.notifications")} />
      }>
        <Bell className="w-5 h-5 text-muted-foreground" />
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 min-w-4 h-4 px-0.5 bg-destructive text-destructive-foreground text-[9px] font-bold rounded-full flex items-center justify-center animate-scale-in">
            {unreadCount > 99 ? "99+" : String(unreadCount)}
          </span>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <div className="px-3 py-2 border-b border-border text-sm font-medium">
          <div className="flex items-center justify-between">
            <span>{t("topbar.notifications")}</span>
            {unreadCount > 0 && (
              <Button variant="ghost" size="xs" onClick={markAllRead} className="text-[10px] text-primary hover:text-primary/80">
                {t("topbar.mark_all_read")}
              </Button>
            )}
          </div>
        </div>
        <ScrollArea className="max-h-64">
          {notifications.length === 0 ? (
            <div className="p-6 text-center text-muted-foreground/70 text-sm">
              <BellOff className="w-6 h-6 mx-auto mb-2" />
              {t("topbar.no_notifications")}
            </div>
          ) : (
            notifications.map((n) => (
              <div key={n.id} className={`px-4 py-3 border-b border-border last:border-0 ${!n.read ? "bg-primary/5" : ""}`}>
                <div className="flex items-start gap-2">
                  <span className={`w-2 h-2 rounded-full mt-1.5 shrink-0 ${typeColors[n.type] || typeColors.info}`} />
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-foreground truncate">{n.message}</p>
                    <p className="text-[10px] text-muted-foreground/70 mt-0.5">{n.time}</p>
                  </div>
                </div>
              </div>
            ))
          )}
        </ScrollArea>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function UserDropdown() {
  const router = useRouter();
  const { t } = useI18n();
  const currentUsername = useAppStore((s) => s.currentUsername);
  const currentUserRole = useAppStore((s) => s.currentUserRole);
  const name = currentUsername || "admin";
  const role = currentUserRole || "Admin";
  const avatar = name.charAt(0).toUpperCase();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={
        <Button variant="ghost" className="flex items-center gap-2 px-2 py-1.5" />
      }>
        <div className="w-8 h-8 bg-gradient-to-br from-primary to-primary/80 rounded-lg flex items-center justify-center text-primary-foreground text-xs font-bold shadow-sm shadow-primary/20 transition-transform duration-150 hover:scale-105">
          {avatar}
        </div>
        <div className="hidden md:block text-left">
          <div className="text-xs font-medium text-foreground">{name}</div>
          <div className="text-[10px] text-muted-foreground/70">{role}</div>
        </div>
        <span className="md:hidden text-xs font-medium text-foreground max-w-[60px] truncate">{name.slice(0, 6)}</span>
        <svg className="w-3 h-3 text-muted-foreground/70 hidden md:block" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M4.18179 6.18181C4.35753 6.00608 4.64245 6.00608 4.81819 6.18181L7.49999 8.86362L10.1818 6.18181C10.3575 6.00608 10.6424 6.00608 10.8182 6.18181C10.9939 6.35755 10.9939 6.64247 10.8182 6.81821L7.81819 9.81821C7.73379 9.9026 7.61934 9.95001 7.49999 9.95001C7.38064 9.95001 7.26618 9.9026 7.18179 9.81821L4.18179 6.81821C4.00605 6.64247 4.00605 6.35755 4.18179 6.18181Z" fill="currentColor" fillRule="evenodd" clipRule="evenodd" /></svg>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <div className="px-3 py-2 border-b border-border text-sm font-medium">
          <div>{name}</div>
          <div className="text-[10px] text-muted-foreground/70">Role: {role}</div>
        </div>
        <DropdownMenuItem onClick={() => router.push("/settings")}>
          <Settings className="w-4 h-4" />{t("topbar.settings")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => router.push("/audit")}>
          <Shield className="w-4 h-4" />{t("topbar.audit_log")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive"
          onClick={() => { api.post("/logout").catch(() => toast.error("Logout failed")).finally(() => router.push("/login")); }}>
          <LogOut className="w-4 h-4" />
          {t("topbar.logout")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export default function TopBar({ onMenuToggle }: { onMenuToggle?: () => void }) {
  const { connected } = useWebSocket();
  const storeSidebarWidth = useAppStore((s) => s.getSidebarWidth());

  return (
    <header
      className="h-14 bg-card/80 backdrop-blur-xl border-b border-border/60 shadow-sm flex items-center justify-between px-3 fixed top-0 right-0 z-30 transition-[left] duration-200 ease-in-out"
      style={{ left: storeSidebarWidth }}
    >
      <div className="flex items-center gap-3 flex-1 min-w-0">
        <Button onClick={onMenuToggle}
          variant="ghost" size="icon" className="lg:hidden" aria-label="Toggle menu">
          <Menu className="w-5 h-5" />
        </Button>
        <SearchBox />
      </div>

      <div className="flex items-center gap-1">
        {/* WS Status */}
        <div className="flex items-center gap-1.5 px-2 py-1 mr-2 shrink-0" title={connected ? "Real-time connected" : "Disconnected"}>
          <span className={`w-2 h-2 rounded-full ${connected ? "bg-emerald-500 animate-pulse" : "bg-red-500"}`} />
          <span className="text-[10px] text-muted-foreground/70 hidden lg:inline">{connected ? "Live" : "Offline"}</span>
        </div>

        <ShortcutsHelpButton />
        <ThemeSelector />
        <LanguageSelector />
        <NotificationDropdown />
        <UserDropdown />
      </div>
    </header>
  );
}
