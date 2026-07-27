"use client";

import { Component, ErrorInfo, ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Bug } from "lucide-react";
import { I18nProvider, useI18n } from "@/lib/i18n";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

class ErrorBoundaryInner extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback;
      return <ErrorFallbackUI error={this.state.error} onReset={() => this.setState({ hasError: false, error: null })} />;
    }
    return this.props.children;
  }
}

function ErrorFallbackUI({ error, onReset }: { error: Error | null; onReset: () => void }) {
  const { t } = useI18n();
  return (
    <div className="flex flex-col items-center justify-center min-h-[300px] p-8 text-center">
      <div className="w-14 h-14 rounded-xl bg-destructive/10 flex items-center justify-center mb-4">
        <Bug className="w-7 h-7 text-destructive" />
      </div>
      <h2 className="text-lg font-semibold text-foreground mb-2">{t("error.boundary_title")}</h2>
      <p className="text-sm text-muted-foreground mb-4 max-w-md">
        {error?.message || t("error.boundary_message")}
      </p>
      <div className="flex gap-2">
        <Button size="sm" variant="outline" onClick={onReset}>
          {t("error.try_again")}
        </Button>
        <Button size="sm" onClick={() => window.location.reload()}>
          {t("error.reload_page")}
        </Button>
      </div>
    </div>
  );
}

export default function ErrorBoundary(props: Props) {
  return (
    <I18nProvider>
      <ErrorBoundaryInner {...props} />
    </I18nProvider>
  );
}

