"use client";

import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { WifiOff } from "lucide-react";
import { Banner } from "@/components/ui/banner";

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
      <div className="mx-auto w-full mt-3">
        <Banner tone="destructive" alert icon={<WifiOff className="size-4" />}>
          {t("network.offline")}
        </Banner>
      </div>
    </div>
  );
}
