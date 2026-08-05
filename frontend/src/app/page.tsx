"use client";

import { useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import { useI18n } from "@/lib/i18n";

export default function Home() {
  const { t } = useI18n();
  const router = useRouter();
  const pathname = usePathname();
  useEffect(() => {
    if (pathname === "/") {
      router.replace("/dashboard");
    }
  }, [router, pathname]);

  if (typeof window === "undefined") return null;

  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="flex flex-col items-center gap-4">
        <div className="w-10 h-10 border-2 border-primary border-t-transparent rounded-full animate-spin" />
        <p className="text-sm text-muted-foreground">{t("common.redirecting")}</p>
      </div>
    </div>
  );
}
