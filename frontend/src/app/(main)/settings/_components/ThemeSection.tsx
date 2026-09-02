"use client";

import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { CheckCircle2, Monitor, Info, Moon, Palette, Sun, Rows3, Rows2 } from "lucide-react";
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
            <Moon className="size-4 text-muted-foreground" />
            {t("settings.theme_mode")}
          </h3>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            {modes.map(({ id, icon: Icon, descKey }) => (
              <Button key={id} onClick={() => onApplyTheme(id)} variant="ghost"
                className={`relative h-auto min-h-32 flex-col items-start justify-start whitespace-normal rounded-xl border p-4 text-left transition-colors ${theme === id ? "border-primary bg-primary/8 shadow-xs" : "border-border bg-muted/45 hover:border-primary/45 hover:bg-primary/5"}`}>
                {theme === id && <CheckCircle2 className="absolute right-3 top-3 size-4 text-primary" />}
                <span className={`mb-3 flex size-9 items-center justify-center rounded-lg bg-card shadow-xs ${id === "light" ? "text-warning" : id === "dark" ? "text-chart-3" : "text-muted-foreground"}`}>
                  <Icon className="size-5" />
                </span>
                <span className="text-sm font-semibold text-foreground">{t(`settings.theme_${id}`)}</span>
                <span className="mt-1 text-xs leading-5 text-muted-foreground">{t(descKey)}</span>
              </Button>
            ))}
          </div>
        </div>
        <div className="bg-muted rounded-lg p-4 mb-6">
          <div className="flex items-center gap-3">
            <Info className="size-4" />
            <div className="text-xs text-muted-foreground">{t("settings.theme_storage_hint")}</div>
          </div>
        </div>

        <div>
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2">
            <Rows2 className="size-4 text-muted-foreground" />
            {t("settings.density")}
          </h3>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {densities.map(({ id, icon: Icon, descKey }) => (
              <Button key={id} onClick={() => setDensity(id)} variant="ghost"
                className={`relative h-auto min-h-28 flex-col items-start justify-start whitespace-normal rounded-xl border p-4 text-left transition-colors ${density === id ? "border-primary bg-primary/8 shadow-xs" : "border-border bg-muted/45 hover:border-primary/45 hover:bg-primary/5"}`}>
                {density === id && <CheckCircle2 className="absolute right-3 top-3 size-4 text-primary" />}
                <span className="mb-3 flex size-9 items-center justify-center rounded-lg bg-card text-chart-3 shadow-xs"><Icon className="size-5" /></span>
                <span className="text-sm font-semibold text-foreground">{t(`settings.density_${id}`)}</span>
                <span className="mt-1 text-xs leading-5 text-muted-foreground">{t(descKey)}</span>
              </Button>
            ))}
          </div>
        </div>
      </div>
    </Card>
  );
}
