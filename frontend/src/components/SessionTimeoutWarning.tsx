"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { buildUrl } from "@/lib/api";

const CHECK_INTERVAL_MS = 60_000;
const WARN_BEFORE_MS = 5 * 60 * 1000;

function getTokenExpiry(): number | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(/(?:^|;\s*)forgec2_session=([^;]*)/);
  if (!match) return null;
  try {
    const payload = JSON.parse(atob(match[1].split(".")[1]));
    return typeof payload.exp === "number" ? payload.exp * 1000 : null;
  } catch {
    return null;
  }
}

export default function SessionTimeoutWarning() {
  const [remaining, setRemaining] = useState<number | null>(null);
  const [visible, setVisible] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const toastShownRef = useRef(false);

  const check = useCallback(() => {
    const exp = getTokenExpiry();
    if (!exp) {
      setVisible(false);
      toastShownRef.current = false;
      return;
    }
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
      toast.warning(`Session expires in ${minutes} minute${minutes !== 1 ? "s" : ""}. Click "Extend Session" to stay logged in.`, {
        duration: 10000,
      });
    }
  }, []);

  useEffect(() => {
    check();
    intervalRef.current = setInterval(check, CHECK_INTERVAL_MS);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [check]);

  const handleExtend = useCallback(async () => {
    try {
      await fetch(buildUrl("/api/me"), { credentials: "include" });
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
        className={`rounded-2xl border p-4 shadow-lg backdrop-blur-sm ${
          urgent
            ? "border-red-300 bg-red-50/95 dark:border-red-700 dark:bg-red-950/95"
            : "border-amber-300 bg-amber-50/95 dark:border-amber-700 dark:bg-amber-950/95"
        }`}
      >
        <p
          className={`text-sm font-medium ${
            urgent
              ? "text-red-800 dark:text-red-200"
              : "text-amber-800 dark:text-amber-200"
          }`}
        >
          {urgent
            ? `Session expires in ${remaining < 60_000 ? "< 1 minute" : `${minutes} minute${minutes !== 1 ? "s" : ""}`}`
            : `Session expires in ${minutes} minute${minutes !== 1 ? "s" : ""}`}
        </p>
        <div className="mt-3 flex gap-2">
          <Button
            size="sm"
            variant={urgent ? "destructive" : "default"}
            onClick={handleExtend}
          >
            Extend Session
          </Button>
          <Button size="sm" variant="ghost" onClick={handleLogout}>
            Logout
          </Button>
        </div>
      </div>
    </div>
  );
}
