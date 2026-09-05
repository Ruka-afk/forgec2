"use client";

import { useEffect, useState } from "react";
import { getRateLimitRetryAfter } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner } from "@/components/ui/banner";

export default function RateLimitBanner() {
  const { t } = useI18n();
  const [retryAfter, setRetryAfter] = useState(0);

  // Slow 5s watch detects newly-set limits (the api layer exposes no event).
  // React bails out when the value is unchanged, so the idle cost is one
  // integer read per tick with no re-render.
  useEffect(() => {
    setRetryAfter(getRateLimitRetryAfter());
    const id = setInterval(() => {
      setRetryAfter(getRateLimitRetryAfter());
    }, 5000);
    return () => clearInterval(id);
  }, []);

  // Smooth 1s countdown only while a limit is actually displayed.
  const rateLimited = retryAfter > 0;
  useEffect(() => {
    if (!rateLimited) return;
    const id = setInterval(() => {
      setRetryAfter(getRateLimitRetryAfter());
    }, 1000);
    return () => clearInterval(id);
  }, [rateLimited]);

  if (retryAfter <= 0) return null;

  return (
    <Banner tone="warning" floating alert>
      {t("common.rate_limited", { seconds: retryAfter })}
    </Banner>
  );
}
