"use client";

import { useEffect, useCallback } from "react";
import Sidebar from "@/components/Sidebar";
import TopBar from "@/components/TopBar";
import UpdateBanner from "@/components/UpdateBanner";
import AgentStatusBanner from "@/components/AgentStatusBanner";
import ShortcutsHelp from "@/components/ShortcutsHelp";
import ScrollToTop from "@/components/ScrollToTop";
import { Breadcrumb } from "@/components/UI";
import { usePathname } from "next/navigation";
import { useAppStore } from "@/lib/store";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isMobile = useAppStore((s) => s.isMobile);
  const setIsMobile = useAppStore((s) => s.setIsMobile);
  const setMobileMenuOpen = useAppStore((s) => s.setMobileMenuOpen);

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

  useEffect(() => {
    setMobileMenuOpen(false);
  }, [pathname, setMobileMenuOpen]);

  const handleMenuToggle = useCallback(() => {
    if (isMobile) setMobileMenuOpen(true);
    else useAppStore.getState().toggleSidebar();
  }, [isMobile, setMobileMenuOpen]);

  return (
    <div className="h-screen flex overflow-hidden bg-background">
      {/* Skip to content link (WCAG 2.4.1) */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[100] focus:bg-primary focus:text-primary-foreground focus:px-4 focus:py-2 focus:rounded-lg focus:text-sm focus:font-medium focus:shadow-lg focus:outline-none"
      >
        Skip to content
      </a>

      <Sidebar />

      <div
        className="flex flex-col flex-1 min-w-0 min-h-0 transition-[margin] duration-200 ease-in-out"
        style={{ marginLeft: isMobile ? 0 : sidebarWidth }}
      >
        <TopBar onMenuToggle={handleMenuToggle} />
        <UpdateBanner />
        <AgentStatusBanner />
        <main id="main-content" tabIndex={-1} className="flex-1 overflow-y-auto min-h-0 pt-14 scroll-smooth focus:outline-none">
          <div className="mx-auto h-full w-full max-w-screen-2xl px-4 sm:px-6 lg:px-8 py-4 sm:py-6 lg:py-8">
            <Breadcrumb />
            {children}
          </div>
        </main>
      </div>

      <ShortcutsHelp />
      <ScrollToTop />
    </div>
  );
}
