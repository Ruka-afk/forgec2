"use client";

import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { Monitor, Info, Moon, Palette, Sun, Rows3, Rows2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useAppStore } from "@/lib/store";

export default function ThemeSection({ theme, onApplyTheme }: { theme: string; onApplyTheme: (t: string) => void }) {
  const { t } = useI18n();
  const density = useAppStore((s) => s.density);
  const setDensity = useAppStore((s) => s.setDensity);
  const modes = [
    { id: "light", icon: Sun, descKey: "settings.theme_light_desc" },
    { id: "dark", icon: Moon, descKey: "settings.theme_dark_desc" },
    { id: "system", icon: Monitor, descKey: "settings.theme_system_desc" },
  ] as const;
  const densities = [
    { id: "comfortable" as const, icon: Rows3, descKey: "settings.density_comfortable_desc" },
    { id: "compact" as const, icon: Rows2, descKey: "settings.density_compact_desc" },
  ];

  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={Palette} tone="primary" title={t("settings.theme_title")} description={t("settings.theme_subtitle")} />
      <div className="p-(--card-spacing)">
        <div className="mb-6">
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2">
            <Moon className="w-4 h-4 text-muted-foreground" />
            {t("settings.theme_mode")}
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            {modes.map(({ id, icon: Icon, descKey }) => (
              <Button key={id} onClick={() => onApplyTheme(id)} variant="ghost"
                className={`p-4 border rounded-lg transition-colors capitalize ${theme === id ? "border-primary bg-primary/10 dark:bg-primary/20" : "bg-muted border-border hover:border-primary hover:bg-primary/10 dark:hover:bg-primary/20"}`}>
                <div className={`text-2xl mb-2 ${id === "light" ? "text-warning" : id === "dark" ? "text-chart-3" : "text-muted-foreground"}`}>
                  <Icon className="w-6 h-6" />
                </div>
                <div className="text-sm font-medium text-muted-foreground">{t(`settings.theme_${id}`)}</div>
                <div className="text-xs text-muted-foreground mt-1">{t(descKey)}</div>
              </Button>
            ))}
          </div>
        </div>
        <div className="bg-muted rounded-lg p-4 mb-6">
          <div className="flex items-center gap-3">
            <Info className="w-4 h-4" />
            <div className="text-xs text-muted-foreground">{t("settings.theme_storage_hint")}</div>
          </div>
        </div>

        <div>
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2">
            <Rows2 className="w-4 h-4 text-muted-foreground" />
            {t("settings.density")}
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {densities.map(({ id, icon: Icon, descKey }) => (
              <Button key={id} onClick={() => setDensity(id)} variant="ghost"
                className={`p-4 border rounded-lg transition-colors capitalize ${density === id ? "border-primary bg-primary/10 dark:bg-primary/20" : "bg-muted border-border hover:border-primary hover:bg-primary/10 dark:hover:bg-primary/20"}`}>
                <div className="text-2xl mb-2 text-chart-3">
                  <Icon className="w-6 h-6" />
                </div>
                <div className="text-sm font-medium text-muted-foreground">{t(`settings.density_${id}`)}</div>
                <div className="text-xs text-muted-foreground mt-1">{t(descKey)}</div>
              </Button>
            ))}
          </div>
        </div>
      </div>
    </Card>
  );
}
