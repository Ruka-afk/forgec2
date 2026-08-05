"use client";

import React from "react";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Eye, EyeOff, ShieldCheck, Wand2 } from "lucide-react";

interface QuickPresetsProps {
  onApply: (preset: "opsec" | "evasion" | "blend") => void;
}

const PRESETS = [
  {
    key: "opsec" as const,
    icon: <ShieldCheck className="w-4 h-4" />,
    iconBg: "bg-green-500/10",
    iconColor: "text-emerald-500",
    labelKey: "generate.preset_opsec",
    descKey: "generate.preset_opsec_desc",
  },
  {
    key: "evasion" as const,
    icon: <Eye className="w-4 h-4" />,
    iconBg: "bg-red-500/10",
    iconColor: "text-destructive",
    labelKey: "generate.preset_evasion",
    descKey: "generate.preset_evasion_desc",
  },
  {
    key: "blend" as const,
    icon: <EyeOff className="w-4 h-4" />,
    iconBg: "bg-blue-500/10",
    iconColor: "text-blue-600 dark:text-blue-400",
    labelKey: "generate.preset_blend",
    descKey: "generate.preset_blend_desc",
  },
];

export default React.memo(function QuickPresets({ onApply }: QuickPresetsProps) {
  const { t } = useI18n();
  return (
    <div className="flex items-center gap-x-3 gap-y-4 flex-wrap">
      <div className="flex items-center gap-x-2.5 shrink-0">
        <div className="w-9 h-9 bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 rounded-xl flex items-center justify-center"><Wand2 className="w-4 h-4" /></div>
        <div>
          <div className="text-sm font-semibold text-foreground">{t("generate.quick_presets")}</div>
          <div className="text-xs text-muted-foreground">{t("generate.quick_presets_desc")}</div>
        </div>
      </div>
      <div className="flex flex-1 flex-wrap gap-3 min-w-0">
        {PRESETS.map((p) => (
          <Card key={p.key} className="relative flex-1 min-w-[180px] p-3.5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
            <button
              type="button"
              onClick={() => onApply(p.key)}
              className="absolute inset-0 z-10 rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1"
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
