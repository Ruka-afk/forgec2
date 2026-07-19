"use client";

import { API_BASE } from "@/lib/constants";
import { PageHeader } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { BookOpen, Download } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function DocsPage() {
  const { t } = useI18n();
  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up h-[calc(100vh-8rem)]">
      <div className="flex items-center gap-3 mb-4">
        <div className="w-10 h-10 bg-gradient-to-br from-sky-500 to-indigo-600 rounded-xl flex items-center justify-center shadow-lg shadow-sky-500/20">
          <BookOpen className="w-4 h-4" />
        </div>
        <div>
          <PageHeader title={t("docs.title")} />
          <p className="text-muted-foreground text-xs">{t("docs.subtitle")}</p>
        </div>
        <Button
          variant="outline"
          size="xs"
          className="ml-auto"
          render={
            <a
              href={`${API_BASE}/docs/openapi.yaml`}
              target="_blank"
              rel="noopener noreferrer"
            />
          }
        >
          <Download className="w-4 h-4" />{t("docs.openapi_yaml")}
        </Button>
      </div>
      <Card className="overflow-hidden h-[calc(100%-4rem)]">
        <iframe
          src={`${API_BASE}/docs/`}
          title="ForgeC2 API Docs"
          className="w-full h-full border-0"
        />
      </Card>
    </div>
  );
}