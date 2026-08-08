import type { Metadata } from "next";
import Script from "next/script";
import localFont from "next/font/local";
import "./globals.css";
import ClientProvider from "@/components/ClientProvider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "sonner";
import { cn } from "@/lib/utils";

const inter = localFont({
  src: "./fonts/Inter-Variable.ttf",
  variable: "--font-inter",
  weight: "100 900",
  display: "swap",
});

const jetBrainsMono = localFont({
  src: "./fonts/JetBrainsMono-Variable.ttf",
  variable: "--font-jbmono",
  weight: "100 900",
  display: "swap",
});

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
    <html
      lang="zh"
      suppressHydrationWarning
      className={cn("h-full", "font-sans", inter.variable, jetBrainsMono.variable)}
    >
      <head>
        <meta charSet="utf-8" />
        <Script id="theme-init" strategy="beforeInteractive">{`
          (function(){
            try {
              var t = localStorage.getItem('forgec2_theme');
              var dark;
              if (t === 'dark') dark = true;
              else if (t === 'light') dark = false;
              else if (t === 'system') dark = window.matchMedia('(prefers-color-scheme: dark)').matches;
              else dark = true; /* new users default to dark */
              if (dark) document.documentElement.classList.add('dark');
            } catch(e){ /* silent */ }
          })();
        `}</Script>

      </head>
      <body className="antialiased h-full">
        <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-(--z-skip-link) focus:bg-primary focus:text-primary-foreground focus:px-4 focus:py-2 focus:rounded-lg focus:shadow-lg focus:outline-none">
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
