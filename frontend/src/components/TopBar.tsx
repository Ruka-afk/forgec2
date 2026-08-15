"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { useWebSocket } from "@/lib/useWebSocket";
import { useI18n } from "@/lib/i18n";
import { useTheme, type Theme } from "@/lib/theme";
import { useAppStore } from "@/lib/store";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { ShortcutsHelpButton } from "@/components/ShortcutsHelp";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { AvatarFallback } from "@/components/ui/avatar";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ConnectionDot } from "@/components/ui/connection-dot";
import {
  Menu, Search, X, Moon, Sun, Monitor, Bell, BellOff,
  Settings, Shield, LogOut, ChevronDown, AlertTriangle,
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

  return (
    <>
      {/* Desktop search */}
      <form onSubmit={handleSubmit} className="relative flex-1 max-w-sm hidden sm:flex">
        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground/70" />
        <Input id="global-search" type="text" value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("topbar.search_placeholder")}
          className="h-9 pl-8 pr-4 text-(--fs-compact) placeholder:text-muted-foreground/70 bg-secondary/50 border-transparent focus:bg-card focus:border-border rounded-xl" />
        {query && (
          <Button type="button" onClick={() => setQuery("")}
            variant="ghost" size="icon-xs" className="absolute right-2 top-1/2 -translate-y-1/2" aria-label={t("common.clear_search")}>
            <X className="w-3 h-3" />
          </Button>
        )}
      </form>
      {/* Mobile search toggle */}
      <Button variant="ghost" size="icon" className="sm:hidden" onClick={() => setMobileOpen(!mobileOpen)} aria-label={t("nav.search")}>
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
                className="absolute right-2 top-1/2 -translate-y-1/2" aria-label={t("common.clear_search")}>
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
      <Tooltip>
        <TooltipTrigger render={<SelectTrigger size="sm" className="w-8 h-8 p-0 justify-center" aria-label={t("topbar.theme")}>
            <SelectValue>
              <ThemeIcon className="w-4 h-4" />
            </SelectValue>
          </SelectTrigger>} />
        <TooltipContent>{t("topbar.theme")}</TooltipContent>
      </Tooltip>
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
  const { locale, setLocale, t } = useI18n();
  const currentLang = LANG_OPTIONS.find((l) => l.value === locale) || LANG_OPTIONS[0];

  return (
    <Select value={locale} onValueChange={(v) => setLocale(v as "en" | "zh")}>
      <Tooltip>
        <TooltipTrigger render={<SelectTrigger size="sm" className="w-8 h-8 p-0 justify-center" aria-label={t("common.language")}>
            <SelectValue>
              <span className="text-sm">{currentLang.flag}</span>
            </SelectValue>
          </SelectTrigger>} />
        <TooltipContent>{t("common.language")}</TooltipContent>
      </Tooltip>
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

function formatNotifTime(raw: string): string {
  const d = new Date(raw);
  if (raw && !isNaN(d.getTime())) return d.toLocaleString();
  return raw || new Date().toLocaleTimeString();
}

function NotificationDropdown() {
  const { t } = useI18n();
  const [notifications, setNotifications] = useState<Notification[]>([]);

  const loadNotifications = useCallback(() => {
    api.get(paths.notifications.list("page=1&pageSize=20"))
      .then((data) => {
        const list = (data.notifications || data.data || []) as Array<Record<string, unknown>>;
        const mapped: Notification[] = list.slice(0, 20).map((n) => ({
          id: Number(n.id) || 0,
          type: (["info", "warning", "error", "success"].includes(String(n.severity || n.type || "info"))
            ? String(n.severity || n.type || "info")
            : "info") as Notification["type"],
          message: String(n.message || n.title || ""),
          time: formatNotifTime(String(n.created_at || "")),
          read: Boolean(n.read),
        }));
        setNotifications(mapped);
      })
      .catch(() => { /* silent */ });
  }, []);

  useEffect(() => {
    loadNotifications();
  }, [loadNotifications]);

  const pushNotification = useCallback((type: Notification["type"], message: string) => {
    const id = notifSeq++;
    setNotifications((prev) => [
      { id, type, message, time: new Date().toLocaleTimeString(), read: false },
      ...prev.slice(0, 49),
    ]);
  }, []);

  // Coalesce bursty task_update WS events into a single notification per
  // status/task-type within a 4s window, so a large task batch does not flood
  // the dropdown with dozens of entries.
  const taskPendingRef = useRef<Map<string, { status: string; taskType: string; cmd: string; count: number }>>(new Map());
  const taskFlushRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const flushTaskNotifications = useCallback(() => {
    taskFlushRef.current = null;
    const pending = taskPendingRef.current;
    taskPendingRef.current = new Map();
    for (const item of pending.values()) {
      if (item.status === "completed") {
        pushNotification(
          "success",
          item.count === 1
            ? t("topbar.notif.task_done", { type: item.taskType, cmd: item.cmd })
            : t("topbar.notif.task_done_multi", { count: item.count }),
        );
      } else {
        pushNotification(
          "error",
          item.count === 1
            ? t("topbar.notif.task_failed", { type: item.taskType, cmd: item.cmd })
            : t("topbar.notif.task_failed_multi", { count: item.count }),
        );
      }
    }
  }, [pushNotification, t]);

  const queueTaskNotification = useCallback((status: string, taskType: string, cmd: string) => {
    const key = `${status}:${taskType}`;
    const existing = taskPendingRef.current.get(key);
    if (existing) {
      existing.count += 1;
    } else {
      taskPendingRef.current.set(key, { status, taskType, cmd, count: 1 });
    }
    if (!taskFlushRef.current) {
      taskFlushRef.current = setTimeout(flushTaskNotifications, 4000);
    }
  }, [flushTaskNotifications]);

  useEffect(() => () => {
    if (taskFlushRef.current) clearTimeout(taskFlushRef.current);
  }, []);

  const handleWSMessage = useCallback((msg: { type: string; [key: string]: unknown }) => {
    const name = String(msg.hostname || msg.agent_id || "").slice(0, 32);
    if (msg.type === "agent_online") pushNotification("success", t("topbar.notif.agent_online", { name }));
    else if (msg.type === "agent_offline") pushNotification("warning", t("topbar.notif.agent_offline", { name }));
    else if (msg.type === "task_update") {
      const status = String(msg.status || "");
      const type = String(msg.task_type || "");
      const cmd = String(msg.command || "").slice(0, 40);
      if (status === "completed" || status === "failed") queueTaskNotification(status, type, cmd);
    } else if (msg.type === "credential_found") pushNotification("success", t("topbar.notif.credential_found"));
    else if (msg.type === "system_alert") pushNotification("warning", String(msg.message || msg.title || t("topbar.notif.system_alert")));
    else if (msg.type === "update_available") pushNotification("info", t("topbar.notif.update_available", { version: String(msg.latest || "") }));
  }, [pushNotification, queueTaskNotification, t]);

  useWebSocket(handleWSMessage);

  const unreadCount = notifications.filter((n) => !n.read).length;
  const markAllRead = () => {
    setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));
    api.put(paths.notifications.readAll).catch(() => { /* silent */ });
  };

  const typeColors: Record<string, string> = {
    success: "bg-success",
    warning: "bg-warning",
    error: "bg-destructive",
    info: "bg-info",
  };

  return (
    <DropdownMenu onOpenChange={(next) => { if (next) loadNotifications(); }}>
      <DropdownMenuTrigger render={
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="icon" className="relative" aria-label={t("topbar.notifications")} />}>
            <Bell className="w-5 h-5 text-muted-foreground" />
            {unreadCount > 0 && (
              <Badge variant="destructive" className="absolute -top-0.5 -right-0.5 min-w-4 h-4 px-0.5 text-(--fs-micro) font-bold rounded-full flex items-center justify-center animate-scale-in">
                {unreadCount > 99 ? "99+" : String(unreadCount)}
              </Badge>
            )}
          </TooltipTrigger>
          <TooltipContent>{t("topbar.notifications")}</TooltipContent>
        </Tooltip>
      }>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <div className="px-3 py-2 border-b border-border text-sm font-medium">
          <div className="flex items-center justify-between">
            <span>{t("topbar.notifications")}</span>
            {unreadCount > 0 && (
              <Button variant="ghost" size="xs" onClick={markAllRead} className="text-(--fs-micro-sm) text-primary hover:text-primary/80">
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
                    <p className="text-(--fs-micro-sm) text-muted-foreground/70 mt-0.5">{n.time}</p>
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

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={
        <Button variant="ghost" className="flex items-center gap-2 px-2 py-1.5" />
      }>
        <div className="transition-transform duration-150 hover:scale-105">
          <AvatarFallback
            name={name}
            size="md"
            shape="square"
            className="bg-gradient-to-br from-primary to-primary/80 shadow-sm shadow-primary/20 text-primary-foreground"
          />
        </div>
        <div className="hidden md:block text-left">
          <div className="text-xs font-medium text-foreground">{name}</div>
          <div className="text-(--fs-micro-sm) text-muted-foreground/70">{role}</div>
        </div>
        <span className="md:hidden text-xs font-medium text-foreground max-w-[60px] truncate">{name.slice(0, 6)}</span>
        <ChevronDown className="w-3 h-3 text-muted-foreground/70 hidden md:block" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <div className="px-3 py-2 border-b border-border text-sm font-medium">
          <div>{name}</div>
          <div className="text-(--fs-micro-sm) text-muted-foreground/70">{t("topbar.role", { role })}</div>
        </div>
        <DropdownMenuItem onClick={() => router.push("/settings")}>
          <Settings className="w-4 h-4" />{t("topbar.settings")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => router.push("/audit")}>
          <Shield className="w-4 h-4" />{t("topbar.audit_log")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive"
          onClick={() => { api.post(paths.auth.logout).catch(() =>         toast.error(t("topbar.toast.logout_failed"))).finally(() => router.push("/login")); }}>
          <LogOut className="w-4 h-4" />
          {t("topbar.logout")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export default function TopBar({ onMenuToggle }: { onMenuToggle?: () => void }) {
  const { t } = useI18n();
  const { connected, reconnectFailed, reconnect } = useWebSocket();
  const storeSidebarWidth = useAppStore((s) => s.getSidebarWidth());

  return (
    <>
    <header
      className="h-14 bg-card/80 backdrop-blur-xl border-b border-border/60 shadow-sm flex items-center justify-between px-3 fixed top-0 right-0 z-30 transition-[left] duration-200 ease-in-out"
      style={{ left: storeSidebarWidth }}
    >
      <div className="flex items-center gap-3 flex-1 min-w-0">
        <Button onClick={onMenuToggle}
          variant="ghost" size="icon" className="lg:hidden" aria-label={t("common.toggle_menu")}>
          <Menu className="w-5 h-5" />
        </Button>
        <SearchBox />
      </div>

      <div className="flex items-center gap-1">
        {/* WS Status */}
        <div className="flex items-center gap-2 px-2.5 py-1 mr-2 shrink-0 rounded-full bg-secondary/60 dark:bg-secondary/40 border border-border/50">
          <Tooltip>
            <TooltipTrigger>
              <span role="status" aria-live="polite" className="flex items-center gap-2">
                <ConnectionDot connected={connected} reconnectFailed={reconnectFailed} />
                <span className="text-(--fs-micro-sm) text-muted-foreground/70 hidden lg:inline">{connected ? t("common.live") : reconnectFailed ? t("common.offline") : t("topbar.reconnecting")}</span>
              </span>
            </TooltipTrigger>
            <TooltipContent>{connected ? t("topbar.ws_connected") : reconnectFailed ? t("topbar.ws_lost") : t("topbar.reconnecting")}</TooltipContent>
          </Tooltip>
        </div>

        <ShortcutsHelpButton />
        <ThemeSelector />
        <LanguageSelector />
        <NotificationDropdown />
        <UserDropdown />
      </div>
    </header>
    {reconnectFailed && (
      <div
        className="fixed top-14 right-0 z-20 flex items-center gap-2 px-4 py-2 bg-destructive/10 border-b border-destructive/30 text-destructive text-sm"
        style={{ left: storeSidebarWidth }}
      >
        <AlertTriangle className="w-4 h-4 shrink-0" />
        <span>{t("topbar.ws_disconnected_banner")}</span>
        <Button variant="outline" size="sm" className="ml-auto h-7 text-xs border-destructive/30 text-destructive hover:bg-destructive/10" onClick={reconnect}>
          {t("topbar.reconnect")}
        </Button>
      </div>
    )}
    </>
  );
}
