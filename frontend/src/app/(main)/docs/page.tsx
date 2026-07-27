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
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up h-[calc(100vh-8rem)]">
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
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-4">
        <Card className="p-3 text-xs">
          <div className="font-semibold text-foreground mb-1">{t("docs.cap_matrix_title")}</div>
          <p className="text-muted-foreground mb-2">{t("docs.cap_matrix_desc")}</p>
          <code className="text-(--font-size-xs-sm) font-mono text-primary">docs/CAPABILITY_MATRIX.md</code>
        </Card>
        <Card className="p-3 text-xs">
          <div className="font-semibold text-foreground mb-1">{t("docs.transport_e2e_title")}</div>
          <p className="text-muted-foreground mb-2">{t("docs.transport_e2e_desc")}</p>
          <code className="text-(--font-size-xs-sm) font-mono text-primary">docs/TRANSPORT_E2E.md</code>
        </Card>
      </div>
      <Card className="overflow-hidden h-[calc(100%-8rem)]">
        <iframe
          src={`${API_BASE}/docs/`}
          title="ForgeC2 API Docs"
          className="w-full h-full border-0"
        />
      </Card>
    </div>
  );
}