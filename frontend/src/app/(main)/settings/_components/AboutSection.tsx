import { SettingsData } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { API_BASE } from "@/lib/constants";
import { BookOpen, Cpu, Download, RotateCw } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function AboutSection({
  data, onCheckUpdate,
}: {
  data: SettingsData;
  onCheckUpdate: () => void;
}) {
  const { t } = useI18n();

  return (
    <Card className="overflow-hidden">
      <div className="bg-secondary/60 border-b border-border px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-lg flex items-center justify-center"><Cpu className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">{t("settings.about.system_information")}</h2><p className="text-xs text-muted-foreground">{t("settings.about.runtime_details")}</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          {[
            { label: t("settings.about.version"), value: `v${data.server_version ?? "2.0.0"}` },
            { label: t("settings.about.uptime"), value: data.uptime ?? "-" },
            { label: t("settings.about.go_version"), value: data.go_version ?? "-" },
            { label: t("settings.about.platform"), value: `${data.goos ?? "-"} / ${data.goarch ?? "-"}` },
            { label: t("settings.about.goroutines"), value: data.goroutines ?? "-" },
            { label: t("settings.about.memory_alloc"), value: data.alloc_mem ? (data.alloc_mem / 1024 / 1024).toFixed(1) + " MB" : "-" },
            { label: t("settings.about.total_alloc"), value: data.total_alloc_mem ? (data.total_alloc_mem / 1024 / 1024).toFixed(1) + " MB" : "-" },
            { label: t("settings.about.cpu_cores"), value: data.num_cpu ?? "-" },
            { label: t("settings.about.implants"), value: t("settings.about.implants_value", { total: data.total_agents ?? 0, online: data.online_agents ?? 0 }) },
          ].map((stat) => (
            <div key={stat.label} className={`bg-muted rounded-lg p-4 border border-border ${stat.label === t("settings.about.implants") ? "col-span-2" : ""}`}>
              <div className="text-xs text-muted-foreground">{stat.label}</div>
              <div className="font-medium text-foreground mt-1 font-mono text-xs">{stat.value}</div>
            </div>
          ))}
          <div className="bg-muted rounded-lg p-4 border border-border col-span-2">
            <div className="text-xs text-muted-foreground">{t("settings.about.data_directory")}</div>
            <div className="font-medium text-foreground mt-1 font-mono text-xs truncate">{data.data_dir ?? "-"}</div>
          </div>
        </div>
        <div className="mt-6 flex flex-wrap gap-3">
          <Button onClick={onCheckUpdate} size="lg" className="px-4 bg-primary/10 dark:bg-primary/20 hover:bg-chart-3/20 dark:hover:bg-chart-3 text-primary dark:text-primary text-xs font-medium transition-colors">
            <RotateCw className="w-4 h-4" />{t("settings.about.check_updates")}
          </Button>
            <Button
              variant="outline"
              size="lg"
              className="px-4 gap-1.5 text-xs"
            render={
              <a href={`${API_BASE}/docs/`} target="_blank" rel="noopener noreferrer" />
            }
          >
            <BookOpen className="w-4 h-4" />{t("settings.about.api_docs")}
          </Button>
            <Button
              variant="outline"
              size="lg"
              className="px-4 gap-1.5 text-xs"
            render={
              <a href={`${API_BASE}/docs/openapi.yaml`} target="_blank" rel="noopener noreferrer" />
            }
          >
            <Download className="w-4 h-4" />{t("settings.about.openapi_yaml")}
          </Button>
        </div>
        <div className="mt-6 text-center text-xs text-muted-foreground">
          {t("settings.about.version_label", { version: data.server_version ?? "2.0.0" })}
          <span className="block mt-1">{t("settings.about.authorized_only")}</span>
        </div>
      </div>
    </Card>
  );
}

