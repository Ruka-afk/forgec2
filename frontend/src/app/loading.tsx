"use client";
import { Spinner } from "@/components/ui/spinner";
import { useI18n } from "@/lib/i18n";

export default function RootLoading() {
  const { t } = useI18n();
  return (
    <div className="flex items-center justify-center h-screen w-screen bg-background">
      <div className="flex flex-col items-center gap-4">
        <Spinner size="lg" />
        <p className="text-sm text-muted-foreground">{t("common.loading_app")}</p>
      </div>
    </div>
  );
}
