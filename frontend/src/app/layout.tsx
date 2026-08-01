import type { Metadata } from "next";
import Script from "next/script";
import "./globals.css";
import ClientProvider from "@/components/ClientProvider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "sonner";
import { cn } from "@/lib/utils";

export const metadata: Metadata = {
  title: "ForgeC2 - Professional Red Team C2 Framework",
  description: "Next.js frontend for ForgeC2",
};

export const viewport = {
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" suppressHydrationWarning className={cn("h-full", "font-sans")}>
      <head>
        <meta charSet="utf-8" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet" />
        <Script id="theme-init" strategy="beforeInteractive">{`
          (function(){
            try {
              var t = localStorage.getItem('forgec2_theme');
              var dark = t === 'dark' || (t !== 'light' && window.matchMedia('(prefers-color-scheme: dark)').matches);
              if (dark) document.documentElement.classList.add('dark');
            } catch(e){ /* silent */ }
          })();
        `}</Script>

      </head>
      <body className="antialiased h-full">
        <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[100] focus:bg-primary focus:text-primary-foreground focus:px-4 focus:py-2 focus:rounded-lg focus:shadow-lg focus:outline-none">
          Skip to main content
        </a>
        <TooltipProvider>
          <ClientProvider>{children}</ClientProvider>
        </TooltipProvider>
        <div role="status" aria-live="polite" aria-atomic="false" className="sr-only" id="toast-announcer" />
        <Toaster />
      </body>
    </html>
  );
}
