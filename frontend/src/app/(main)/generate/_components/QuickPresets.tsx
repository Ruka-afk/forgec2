"use client";

import React from "react";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Eye, EyeOff, ShieldCheck, Wand2 } from "lucide-react";

interface QuickPresetsProps {
  onApply: (preset: "opsec" | "evasion" | "blend") => void;
}

const PRESETS = [
  {
    key: "opsec" as const,
    icon: <ShieldCheck className="w-4 h-4" />,
    iconBg: "bg-success/10",
    iconColor: "text-success",
    labelKey: "generate.preset_opsec",
    descKey: "generate.preset_opsec_desc",
  },
  {
    key: "evasion" as const,
    icon: <Eye className="w-4 h-4" />,
    iconBg: "bg-destructive/10",
    iconColor: "text-destructive",
    labelKey: "generate.preset_evasion",
    descKey: "generate.preset_evasion_desc",
  },
  {
    key: "blend" as const,
    icon: <EyeOff className="w-4 h-4" />,
    iconBg: "bg-info/10",
    iconColor: "text-info",
    labelKey: "generate.preset_blend",
    descKey: "generate.preset_blend_desc",
  },
];

export default React.memo(function QuickPresets({ onApply }: QuickPresetsProps) {
  const { t } = useI18n();
  return (
    <div className="flex items-center gap-x-3 gap-y-4 flex-wrap">
      <div className="flex items-center gap-x-2.5 shrink-0">
        <div className="w-9 h-9 bg-chart-2/10 text-info rounded-lg flex items-center justify-center"><Wand2 className="w-4 h-4" /></div>
        <div>
          <div className="text-sm font-semibold text-foreground">{t("generate.quick_presets")}</div>
          <div className="text-xs text-muted-foreground">{t("generate.quick_presets_desc")}</div>
        </div>
      </div>
      <div className="flex flex-1 flex-wrap gap-3 min-w-0">
        {PRESETS.map((p) => (
          <Card key={p.key} className="relative flex-1 min-w-[180px] p-4 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
            <Button
              type="button"
              variant="ghost"
              onClick={() => onApply(p.key)}
              className="absolute inset-0 z-10 h-auto rounded-lg"
              aria-label={t(p.labelKey)}
            />
            <div className="flex items-center gap-2.5">
              <div className={`w-8 h-8 ${p.iconBg} rounded-lg flex items-center justify-center`}><span className={p.iconColor}>{p.icon}</span></div>
              <div className="min-w-0">
                <div className="text-sm font-semibold text-foreground">{t(p.labelKey)}</div>
                <p className="truncate text-xs text-muted-foreground">{t(p.descKey)}</p>
              </div>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
});
