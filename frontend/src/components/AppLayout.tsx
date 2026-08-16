"use client";

import { useEffect, useCallback } from "react";
import Sidebar from "@/components/Sidebar";
import TopBar from "@/components/TopBar";
import UpdateBanner from "@/components/UpdateBanner";
import AgentStatusBanner from "@/components/AgentStatusBanner";
import ShortcutsHelp from "@/components/ShortcutsHelp";
import ScrollToTop from "@/components/ScrollToTop";
import CommandPalette from "@/components/CommandPalette";
import GlobalInteractDock from "@/components/GlobalInteractDock";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { usePathname } from "next/navigation";
import { useAppStore } from "@/lib/store";
import { useI18n } from "@/lib/i18n";
import { getPageTitleKey } from "@/lib/navigation";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { t } = useI18n();
  const isMobile = useAppStore((s) => s.isMobile);
  const setIsMobile = useAppStore((s) => s.setIsMobile);
  const setMobileMenuOpen = useAppStore((s) => s.setMobileMenuOpen);
  const focusMode = useAppStore((s) => s.focusMode);

  const sidebarWidth = useAppStore((s) => s.getSidebarWidth());

  useEffect(() => {
    const check = () => setIsMobile(window.innerWidth < 1024);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, [setIsMobile]);

  useEffect(() => {
    if (isMobile) useAppStore.setState({ sidebarCollapsed: true });
  }, [isMobile]);

  // Focus mode shortcut: Cmd/Ctrl + "." toggles the full-viewport console.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
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
    <div className="h-screen flex overflow-hidden bg-background">
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
        <main id="main-content" tabIndex={-1} className="flex-1 overflow-y-auto min-h-0 pt-14 scroll-smooth focus:outline-none">
          {/* Sticky breadcrumb dock — merges into the topbar chrome plane */}
          {!focusMode && (
            <div className="sticky top-14 z-20 bg-background/85 backdrop-blur-xl border-b border-border/40">
              <div className="mx-auto w-full max-w-screen-2xl px-5 sm:px-8 lg:px-10 py-2">
                <Breadcrumb />
              </div>
            </div>
          )}
            <div className={`mx-auto h-full w-full max-w-screen-2xl px-5 sm:px-8 lg:px-10 py-5 sm:py-7 lg:py-8 ${focusMode ? "pt-5" : ""}`}>
            <UpdateBanner />
            <AgentStatusBanner />
            {children}
          </div>
        </main>
        <GlobalInteractDock />
      </div>

      <ShortcutsHelp />
      <ScrollToTop />
      <CommandPalette />
    </div>
  );
}
