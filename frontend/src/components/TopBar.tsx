"use client";

import { useWebSocket } from "@/lib/useWebSocket";
import { useI18n } from "@/lib/i18n";
import { useAppStore, selectSidebarWidth } from "@/lib/store";
import { ShortcutsHelpButton } from "@/components/ShortcutsHelp";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ConnectionDot } from "@/components/ui/connection-dot";
import { SearchBox } from "@/components/topbar/search-box";
import { ThemeSelector } from "@/components/topbar/theme-selector";
import { LanguageSelector } from "@/components/topbar/language-selector";
import { NotificationDropdown } from "@/components/topbar/notification-dropdown";
import { UserDropdown } from "@/components/topbar/user-dropdown";
import {
  Menu, AlertTriangle, Maximize2, Minimize2,
} from "lucide-react";

export default function TopBar({ onMenuToggle }: { onMenuToggle?: () => void }) {
  const { t } = useI18n();
  const { connected, reconnectFailed, reconnect } = useWebSocket();
  const storeSidebarWidth = useAppStore(selectSidebarWidth);
  const focusMode = useAppStore((s) => s.focusMode);
  const toggleFocusMode = useAppStore((s) => s.toggleFocusMode);

  return (
    <>
    <header
      className="h-14 bg-card/80 backdrop-blur-xl border-b border-border/60 shadow-sm flex items-center justify-between px-3 fixed top-0 right-0 z-30 transition-[left] duration-200 ease-in-out"
      style={{ left: focusMode ? 0 : storeSidebarWidth }}
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

        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="icon" onClick={toggleFocusMode} aria-label={focusMode ? t("topbar.focus_mode_exit") : t("topbar.focus_mode")} />}>
            {focusMode ? <Minimize2 className="w-5 h-5" /> : <Maximize2 className="w-5 h-5" />}
          </TooltipTrigger>
          <TooltipContent>{focusMode ? t("topbar.focus_mode_exit") : t("topbar.focus_mode")}</TooltipContent>
        </Tooltip>
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
        style={{ left: focusMode ? 0 : storeSidebarWidth }}
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