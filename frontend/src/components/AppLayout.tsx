"use client";

import { useEffect } from "react";
import Sidebar from "@/components/Sidebar";
import TopBar from "@/components/TopBar";
import UpdateBanner from "@/components/UpdateBanner";
import ShortcutsHelp from "@/components/ShortcutsHelp";
import { usePathname } from "next/navigation";
import { useAppStore } from "@/lib/store";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isMobile = useAppStore((s) => s.isMobile);
  const setIsMobile = useAppStore((s) => s.setIsMobile);
  const setMobileMenuOpen = useAppStore((s) => s.setMobileMenuOpen);
  const toggleSidebar = useAppStore((s) => s.toggleSidebar);

  useEffect(() => {
    const check = () => setIsMobile(window.innerWidth < 1024);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, [setIsMobile]);

  // Auto-collapse sidebar on mobile
  useEffect(() => {
    if (isMobile) {
      useAppStore.setState({ sidebarCollapsed: true });
    }
  }, [isMobile]);

  // Close mobile menu on navigation
  useEffect(() => {
    setMobileMenuOpen(false);
  }, [pathname, setMobileMenuOpen]);

  return (
    <div className="h-screen flex overflow-hidden" style={{ background: "var(--background)" }}>
      <Sidebar />
      <div className="flex flex-col flex-1 min-w-0 min-h-0">
        <TopBar onMenuToggle={toggleSidebar} />
        <UpdateBanner />
        <main className="flex-1 overflow-y-auto min-h-0 scroll-smooth pt-12">
          <div className="mx-auto w-full px-4 sm:px-6 lg:px-8 py-4 sm:py-6 lg:py-8 animate-fade-in">
            {children}
          </div>
        </main>
      </div>
      <ShortcutsHelp />
    </div>
  );
}
