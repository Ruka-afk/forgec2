"use client";

import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider } from "@/lib/theme";
import { WebSocketProvider } from "@/lib/wsContext";
import ErrorBoundary from "./ErrorBoundary";

export default function ClientProvider({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary>
      <ThemeProvider>
        <I18nProvider>
          <WebSocketProvider>{children}</WebSocketProvider>
        </I18nProvider>
      </ThemeProvider>
    </ErrorBoundary>
  );
}
