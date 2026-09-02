"use client";

import { useEffect, useCallback, useRef } from "react";
import Sidebar from "@/components/Sidebar";
import TopBar from "@/components/TopBar";
import UpdateBanner from "@/components/UpdateBanner";
import AgentStatusBanner from "@/components/AgentStatusBanner";
import ShortcutsHelp from "@/components/ShortcutsHelp";
import ScrollToTop from "@/components/ScrollToTop";
import CommandPalette from "@/components/CommandPalette";
import GlobalInteractDock from "@/components/GlobalInteractDock";
import TelemetryCollector from "@/components/TelemetryCollector";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { usePathname } from "next/navigation";
import { useAppStore, selectSidebarWidth } from "@/lib/store";
import { useI18n } from "@/lib/i18n";
import { getPageTitleKey } from "@/lib/navigation";
import { isFlushPath, showBreadcrumbBar } from "@/lib/layout";
import { cn } from "@/lib/utils";
import { useWS } from "@/lib/wsContext";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { t } = useI18n();
  const isMobile = useAppStore((s) => s.isMobile);
  const setIsMobile = useAppStore((s) => s.setIsMobile);
  const setMobileMenuOpen = useAppStore((s) => s.setMobileMenuOpen);
  const focusMode = useAppStore((s) => s.focusMode);
  const { reconnectFailed } = useWS();

  const sidebarWidth = useAppStore(selectSidebarWidth);
  const flush = isFlushPath(pathname);
  const crumbs = showBreadcrumbBar(pathname, focusMode);

  const prevWidthRef = useRef<number>(typeof window !== "undefined" ? window.innerWidth : 1920);

  useEffect(() => {
    const check = () => {
      const w = window.innerWidth;
      const mobile = w < 1024;
      setIsMobile(mobile);
      // Entering the 1024–1280 compact band: default-collapse the sidebar so
      // narrow laptops get full content width. Leaving the band never fights
      // the operator's explicit toggle (only the crossing triggers it).
      if (!mobile && w < 1280 && prevWidthRef.current >= 1280) {
        useAppStore.setState({ sidebarCollapsed: true });
      }
      prevWidthRef.current = w;
    };
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, [setIsMobile]);

  useEffect(() => {
    if (isMobile) useAppStore.setState({ sidebarCollapsed: true });
  }, [isMobile]);

  // G10 fix: skip e.repeat so holding Ctrl+. does not rapid-toggle focus mode.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.repeat) return;
      if ((e.metaKey || e.ctrlKey) && e.key === ".") {
        e.preventDefault();
        useAppStore.getState().toggleFocusMode();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    setMobileMenuOpen(false);
  }, [pathname, setMobileMenuOpen]);

  // Per-page document.title derived from the route's nav label.
  useEffect(() => {
    if (!pathname || pathname === "/login") return;
    const titleKey = getPageTitleKey(pathname);
    if (!titleKey) return;
    document.title = `${t(titleKey)} — ForgeC2`;
  }, [pathname, t]);

  const handleMenuToggle = useCallback(() => {
    if (isMobile) setMobileMenuOpen(true);
    else useAppStore.getState().toggleSidebar();
  }, [isMobile, setMobileMenuOpen]);

  return (
    <div className="flex h-screen overflow-hidden bg-background supports-[height:100dvh]:h-[100dvh]">
      {/* Skip to content link (WCAG 2.4.1) */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-(--z-skip-link) focus:bg-primary focus:text-primary-foreground focus:px-4 focus:py-2 focus:rounded-lg focus:text-sm focus:font-medium focus:shadow-lg focus:outline-none"
      >
        {t("a11y.skip_to_content")}
      </a>

      {!focusMode && <Sidebar />}

      <div
        className="flex flex-col flex-1 min-w-0 min-h-0 transition-[margin] duration-200 ease-in-out"
        style={{ marginLeft: focusMode || isMobile ? 0 : sidebarWidth }}
      >
        <TopBar onMenuToggle={handleMenuToggle} />
        <main
          id="main-content"
          tabIndex={-1}
          className={cn(
            "relative flex min-h-0 flex-1 flex-col bg-background focus:outline-none",
            reconnectFailed
              ? "pt-[calc(var(--shell-topbar-height)+2.5rem)]"
              : "pt-(--shell-topbar-height)",
            flush ? "overflow-hidden" : "overflow-y-auto scroll-smooth",
          )}
        >
          {crumbs && (
            <div className="sticky top-0 z-20 flex h-(--shell-breadcrumb-height) shrink-0 items-center border-b border-border/70 bg-card/95 backdrop-blur-md">
              <div className="mx-auto w-full max-w-(--content-wide) px-4 sm:px-6 lg:px-8">
                <Breadcrumb />
              </div>
            </div>
          )}
          <div
            className={cn(
              "mx-auto flex w-full min-h-0 flex-col",
              flush
                ? "max-w-none flex-1 px-0 py-0"
                : "max-w-(--content-wide) gap-4 px-4 py-4 sm:px-6 sm:py-5 lg:px-8 lg:py-7",
            )}
          >
            {!flush && (
              <>
                <UpdateBanner />
                <AgentStatusBanner />
              </>
            )}
            <div className={cn(flush && "flex min-h-0 flex-1 flex-col")}>{children}</div>
          </div>
        </main>
        <GlobalInteractDock />
      </div>

      <ShortcutsHelp />
      <ScrollToTop />
      <CommandPalette />
      <TelemetryCollector />
    </div>
  );
}
