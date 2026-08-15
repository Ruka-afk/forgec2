"use client";

import { useEffect, useState } from "react";
import { getRateLimitRetryAfter } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Banner } from "@/components/ui/banner";

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
    <Banner tone="warning" floating>
      {t("common.rate_limited", { seconds: retryAfter })}
    </Banner>
  );
}
