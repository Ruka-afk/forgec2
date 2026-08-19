"use client";

import { useTheme, type Theme } from "@/lib/theme";
import { useI18n } from "@/lib/i18n";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Sun, Moon, Monitor } from "lucide-react";

const THEME_ICONS: Record<Theme, React.ComponentType<{ className?: string }>> = {
  light: Sun,
  dark: Moon,
  system: Monitor,
};

const THEME_OPTIONS: { value: Theme; labelKey: string }[] = [
  { value: "light", labelKey: "topbar.theme_light" },
  { value: "dark", labelKey: "topbar.theme_dark" },
  { value: "system", labelKey: "topbar.theme_system" },
];

export function ThemeSelector() {
  const { theme, setTheme } = useTheme();
  const { t } = useI18n();
  const ThemeIcon = THEME_ICONS[theme];

  return (
    <Select value={theme} onValueChange={(v) => setTheme(v as Theme)}>
      <Tooltip>
        <TooltipTrigger render={<SelectTrigger size="sm" className="w-8 h-8 p-0 justify-center" aria-label={t("topbar.theme")}>
            <SelectValue>
              <ThemeIcon className="w-4 h-4" />
            </SelectValue>
          </SelectTrigger>} />
        <TooltipContent>{t("topbar.theme")}</TooltipContent>
      </Tooltip>
      <SelectContent>
        {THEME_OPTIONS.map((opt) => {
          const Icon = THEME_ICONS[opt.value];
          return (
            <SelectItem key={opt.value} value={opt.value}>
              <Icon className="w-4 h-4" />
              <span>{t(opt.labelKey)}</span>
            </SelectItem>
          );
        })}
      </SelectContent>
    </Select>
  );
}