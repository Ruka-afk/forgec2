"use client";

import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { WifiOff } from "lucide-react";

export default function NetworkStatusBanner() {
  const [online, setOnline] = useState(true);
  const { t } = useI18n();

  useEffect(() => {
    const handleOnline = () => setOnline(true);
    const handleOffline = () => setOnline(false);
    setOnline(navigator.onLine);
    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
    };
  }, []);

  if (online) return null;

  return (
    <div className="px-4 sm:px-6 lg:px-8">
      <div className="mx-auto w-full mt-3 rounded-xl border border-red-400/40 bg-red-50 dark:bg-red-900/20 overflow-hidden" role="alert" aria-live="assertive">
        <div className="flex items-center gap-3 px-4 py-2.5">
          <WifiOff className="w-4 h-4 text-red-600 dark:text-red-400 shrink-0" />
          <span className="text-sm text-red-700 dark:text-red-300 font-medium">
            {t("network.offline")}
          </span>
        </div>
      </div>
    </div>
  );
}
