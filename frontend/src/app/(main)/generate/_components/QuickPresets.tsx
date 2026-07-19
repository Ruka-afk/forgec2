"use client";

import React from "react";
import { Card } from "@/components/ui/card";
import { Eye, EyeOff, ShieldCheck, Wand2 } from "lucide-react";

interface QuickPresetsProps {
  onApply: (preset: "opsec" | "evasion" | "blend") => void;
}

const PRESETS = [
  {
    key: "opsec" as const,
    icon: <ShieldCheck className="w-4 h-4" />,
    iconBg: "bg-green-100 dark:bg-green-900/30",
    iconColor: "text-emerald-500",
    label: "OPSEC Safe",
    desc: "Minimal features, low jitter, no persistence",
  },
  {
    key: "evasion" as const,
    icon: <Eye className="w-4 h-4" />,
    iconBg: "bg-red-100 dark:bg-red-900/30",
    iconColor: "text-destructive",
    label: "Max Evasion",
    desc: "EDR evasion, obfuscation, persistence, high jitter",
  },
  {
    key: "blend" as const,
    icon: <EyeOff className="w-4 h-4" />,
    iconBg: "bg-blue-100 dark:bg-blue-900/30",
    iconColor: "text-blue-600 dark:text-blue-400",
    label: "Blend In",
    desc: "Google UA profile, moderate intervals",
  },
];

export default React.memo(function QuickPresets({ onApply }: QuickPresetsProps) {
  return (
    <div className="mt-8">
      <div className="flex items-center gap-x-3 mb-5">
        <div className="w-10 h-10 bg-cyan-100 dark:bg-cyan-900/30 rounded-xl flex items-center justify-center"><Wand2 className="w-4 h-4" /></div>
        <div>
          <div className="text-sm font-semibold text-foreground">Quick Presets</div>
          <div className="text-xs text-muted-foreground">Apply common configurations with one click</div>
        </div>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {PRESETS.map((p) => (
          <Card key={p.key} className="p-4 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow cursor-pointer" onClick={() => onApply(p.key)}>
            <div className="flex items-center gap-3 mb-2">
              <div className={`w-8 h-8 ${p.iconBg} rounded-lg flex items-center justify-center`}><span className={p.iconColor}>{p.icon}</span></div>
              <span className="text-sm font-semibold text-foreground">{p.label}</span>
            </div>
            <p className="text-xs text-muted-foreground">{p.desc}</p>
          </Card>
        ))}
      </div>
    </div>
  );
});
