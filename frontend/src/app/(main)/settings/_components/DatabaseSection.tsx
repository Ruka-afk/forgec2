import { SettingsData } from "./types";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { Copy, Database, Download, Minimize2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function DatabaseSection({
  data, saving, onVacuum, onBackup, onDownloadDB,
}: {
  data: SettingsData;
  saving: boolean;
  onVacuum: () => void;
  onBackup: () => void;
  onDownloadDB: () => void;
}) {
  const { t } = useI18n();
  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={Database} tone="cyan" title={t("settings.database.title")} description={t("settings.database.subtitle")} />
      <div className="p-(--card-spacing)">
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3 mb-6">
          {[
            { label: t("settings.database.size"), value: data.database_size ? (data.database_size / 1024 / 1024).toFixed(1) + " MB" : "-" },
            { label: t("settings.database.agents"), value: data.total_agents ?? 0 },
            { label: t("settings.database.listeners"), value: data.total_listeners ?? 0 },
            { label: t("settings.database.tasks"), value: data.total_tasks ?? 0 },
            { label: t("settings.database.credentials"), value: data.total_credentials ?? 0 },
            { label: t("settings.database.audit_logs"), value: data.total_audits ?? 0 },
          ].map((stat) => (
            <div key={stat.label} className="bg-muted rounded-lg p-4 border border-border">
              <div className="text-xs text-muted-foreground">{stat.label}</div>
              <div className="font-semibold text-sm text-foreground mt-1">{stat.value}</div>
            </div>
          ))}
        </div>
        <div className="flex flex-wrap gap-3">
          <Button onClick={onVacuum} size="lg" disabled={saving} className="px-4 bg-primary/10 hover:bg-primary/20 text-primary text-sm font-medium transition-colors disabled:opacity-50">
            <Minimize2 className="w-4 h-4" />{t("settings.database.vacuum")}
          </Button>
          <Button onClick={onBackup} size="lg" disabled={saving} className="px-4 bg-accent hover:bg-accent/80 text-accent-foreground text-sm font-medium transition-colors disabled:opacity-50">
            <Copy className="w-4 h-4" />{t("settings.database.backup")}
          </Button>
          <Button onClick={onDownloadDB} size="lg" className="px-4 bg-success/15 hover:bg-success/20 dark:hover:bg-success/60 text-success text-sm font-medium transition-colors">
            <Download className="w-4 h-4" />{t("settings.database.download")}
          </Button>
        </div>
      </div>
    </Card>
  );
}
