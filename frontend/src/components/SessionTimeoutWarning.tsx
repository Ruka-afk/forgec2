"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { bannerSurface } from "@/components/ui/banner";
import { cn } from "@/lib/utils";

const CHECK_INTERVAL_MS = 60_000;
const WARN_BEFORE_MS = 5 * 60 * 1000;

async function fetchSessionExpiry(): Promise<number | null> {
  try {
    // api.get routes 401 through the shared handleUnauthorized (debounced
    // redirect to /login), so an expired session logs the user out instead of
    // silently dropping the warning.
    const body = await api.get<{ session_exp?: number }>(paths.auth.me);
    const exp = body?.session_exp;
    return typeof exp === "number" && exp > 0 ? exp : null;
  } catch {
    return null;
  }
}

export default function SessionTimeoutWarning() {
  const { t } = useI18n();
  const [remaining, setRemaining] = useState<number | null>(null);
  const [visible, setVisible] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const toastShownRef = useRef(false);
  const sessionExpRef = useRef<number | null>(null);

  const check = useCallback(async () => {
    const exp = await fetchSessionExpiry();
    if (!exp) {
      setVisible(false);
      toastShownRef.current = false;
      return;
    }
    sessionExpRef.current = exp;
    const left = exp - Date.now();
    if (left <= 0) {
      setVisible(false);
      toastShownRef.current = false;
      if (typeof window !== "undefined" && window.location.pathname !== "/login") {
        window.location.href = "/login";
      }
      return;
    }
    setRemaining(left);
    const shouldWarn = left < WARN_BEFORE_MS;
    setVisible(shouldWarn);

    if (shouldWarn && !toastShownRef.current) {
      toastShownRef.current = true;
      const minutes = Math.ceil(left / 60_000);
      toast.warning(
        minutes === 1
          ? t("session.toast_minute")
          : t("session.toast_minutes", { minutes }),
        { duration: 10000 },
      );
    }
  }, [t]);

  useEffect(() => {
    check();
    intervalRef.current = setInterval(check, CHECK_INTERVAL_MS);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [check]);

  const handleExtend = useCallback(async () => {
    try {
      await api.post(paths.auth.extend);
      toastShownRef.current = false;
      check();
    } catch {
      window.location.href = "/login";
    }
  }, [check]);

  const handleLogout = useCallback(() => {
    window.location.href = "/login";
  }, []);

  if (!visible || remaining === null) return null;

  const minutes = Math.max(0, Math.ceil(remaining / 60_000));
  const urgent = remaining < 60_000;

  return (
    <div className="fixed bottom-4 right-4 z-50 max-w-sm">
      <div
        className={cn(bannerSurface(urgent ? "destructive" : "warning", "rounded-xl p-4 shadow-lg backdrop-blur-sm"))}
      >
        <p
          className={`text-sm font-medium ${
            urgent ? "text-destructive" : "text-warning-foreground"
          }`}
        >
          {urgent
            ? t("session.expires_less_minute")
            : minutes === 1
              ? t("session.expires_minute")
              : t("session.expires_minutes", { minutes })}
        </p>
        <div className="mt-3 flex gap-2">
          <Button
            size="sm"
            variant={urgent ? "destructive" : "default"}
            onClick={handleExtend}
          >
            {t("session.extend")}
          </Button>
          <Button size="sm" variant="ghost" onClick={handleLogout}>
            {t("session.logout")}
          </Button>
        </div>
      </div>
    </div>
  );
}
