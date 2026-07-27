"use client";

import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider } from "@/lib/theme";
import { WebSocketProvider } from "@/lib/wsContext";
import ErrorBoundary from "./ErrorBoundary";
import SessionTimeoutWarning from "./SessionTimeoutWarning";
import RateLimitBanner from "./RateLimitBanner";
import NetworkStatusBanner from "./NetworkStatusBanner";

export default function ClientProvider({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary>
      <ThemeProvider>
        <I18nProvider>
          <WebSocketProvider>
            {children}
            <SessionTimeoutWarning />
            <RateLimitBanner />
            <NetworkStatusBanner />
          </WebSocketProvider>
        </I18nProvider>
      </ThemeProvider>
    </ErrorBoundary>
  );
}
