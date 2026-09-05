"use client";

import React from "react";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Eye, EyeOff, ShieldCheck, Wand2 } from "lucide-react";
import { IconBadge } from "@/components/ui/icon-badge";

interface QuickPresetsProps {
  onApply: (preset: "opsec" | "evasion" | "blend") => void;
}

const PRESETS = [
  {
    key: "opsec" as const,
    icon: <ShieldCheck className="size-4" />,
    iconBg: "bg-success/10",
    iconColor: "text-success",
    labelKey: "generate.preset_opsec",
    descKey: "generate.preset_opsec_desc",
  },
  {
    key: "evasion" as const,
    icon: <Eye className="size-4" />,
    iconBg: "bg-destructive/10",
    iconColor: "text-destructive",
    labelKey: "generate.preset_evasion",
    descKey: "generate.preset_evasion_desc",
  },
  {
    key: "blend" as const,
    icon: <EyeOff className="size-4" />,
    iconBg: "bg-info/10",
    iconColor: "text-info",
    labelKey: "generate.preset_blend",
    descKey: "generate.preset_blend_desc",
  },
];

export default React.memo(function QuickPresets({ onApply }: QuickPresetsProps) {
  const { t } = useI18n();
  return (
    <div className="rounded-xl border border-border/60 bg-card p-3.5 shadow-sm">
      <div className="mb-3 flex items-center gap-x-2.5">
        <IconBadge icon={Wand2} color="cyan" size="md" />
        <div>
          <div className="text-sm font-semibold tracking-tight text-foreground">{t("generate.quick_presets")}</div>
          <div className="text-xs leading-4 text-muted-foreground">{t("generate.quick_presets_desc")}</div>
        </div>
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        {PRESETS.map((p) => (
          <Card key={p.key} className="group relative overflow-hidden p-4 transition-all duration-200 hover:-translate-y-px hover:border-primary/20 hover:shadow-md">
            <Button
              type="button"
              variant="ghost"
              onClick={() => onApply(p.key)}
              className="absolute inset-0 z-10 h-auto rounded-xl"
              aria-label={t(p.labelKey)}
            />
            <div className="flex items-center gap-2.5">
              <div className={`grid size-9 shrink-0 place-items-center ${p.iconBg} rounded-xl ring-1 ring-border/30 transition-transform duration-200 group-hover:scale-105`}><span className={p.iconColor}>{p.icon}</span></div>
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
