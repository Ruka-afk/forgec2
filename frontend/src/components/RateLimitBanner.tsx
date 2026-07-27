"use client";

import { useEffect, useState } from "react";
import { getRateLimitRetryAfter } from "@/lib/api";

export default function RateLimitBanner() {
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
      <div className="rounded-2xl border border-orange-300 bg-orange-50/95 px-4 py-2 shadow-lg backdrop-blur-sm dark:border-orange-700 dark:bg-orange-950/95">
        <p className="text-xs font-medium text-orange-800 dark:text-orange-200">
          Rate limited — retry in {retryAfter}s
        </p>
      </div>
    </div>
  );
}
