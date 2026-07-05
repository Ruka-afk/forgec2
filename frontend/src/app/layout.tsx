import type { Metadata } from "next";
import Script from "next/script";
import "./globals.css";
import ClientProvider from "@/components/ClientProvider";

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
    <html lang="en" suppressHydrationWarning className="h-full">
      <head>
        <meta charSet="utf-8" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet" />
        <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css" />
        <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/vis/9.1.9/vis-network.min.css" />
        <Script id="theme-init" strategy="beforeInteractive">{`
          (function(){
            try {
              var t = localStorage.getItem('forgec2_theme');
              var dark = t === 'dark' || (t !== 'light' && window.matchMedia('(prefers-color-scheme: dark)').matches);
              if (dark) document.documentElement.classList.add('dark');
            } catch(e){}
          })();
        `}</Script>
        <script src="/js/tailwind.min.js"></script>
        <script dangerouslySetInnerHTML={{ __html: `tailwind.config={darkMode:'class'};` }} />
      </head>
      <body className="antialiased h-full">
        <ClientProvider>{children}</ClientProvider>
      </body>
    </html>
  );
}
