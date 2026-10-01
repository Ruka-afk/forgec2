import { useEffect, useState } from "react";
import { getRateLimitRetryAfter } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { Banner } from "@/components/ui/banner";

export default function RateLimitBanner() {
  const { t } = useI18n();
  const [retryAfter, setRetryAfter] = useState(0);

  // Slow 5s watch detects newly-set limits (the api layer exposes no event).
  // useVisibleInterval pauses this while the tab is hidden.
  useEffect(() => setRetryAfter(getRateLimitRetryAfter()), []);
  useVisibleInterval(() => setRetryAfter(getRateLimitRetryAfter()), 5000);

  // Smooth 1s countdown only while a limit is actually displayed.
  const rateLimited = retryAfter > 0;
  useVisibleInterval(
    () => setRetryAfter(getRateLimitRetryAfter()),
    rateLimited ? 1000 : 0,
  );

  if (retryAfter <= 0) return null;

  return (
    <Banner tone="warning" floating alert>
      {t("common.rate_limited", { seconds: retryAfter })}
    </Banner>
  );
}
