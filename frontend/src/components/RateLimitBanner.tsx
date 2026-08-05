"use client";

import { useEffect, useState } from "react";
import { getRateLimitRetryAfter } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

export default function RateLimitBanner() {
  const { t } = useI18n();
  const [retryAfter, setRetryAfter] = useState(0);

  useEffect(() => {
    const id = setInterval(() => {
      setRetryAfter(getRateLimitRetryAfter());
    }, 1000);
    return () => clearInterval(id);
  }, []);

  if (retryAfter <= 0) return null;

  return (
    <div className="fixed top-16 left-1/2 z-50 -translate-x-1/2">
      <div className="rounded-2xl border border-warning/40 bg-warning/10 px-4 py-2 shadow-lg backdrop-blur-sm">
        <p className="text-xs font-medium text-warning-foreground">
          {t("common.rate_limited", { seconds: retryAfter })}
        </p>
      </div>
    </div>
  );
}
