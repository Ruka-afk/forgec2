"use client";

import { useI18n } from "@/lib/i18n";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

const LANG_OPTIONS = [
  { value: "en", flag: "\u{1F1FA}\u{1F1F8}", name: "English" },
  { value: "zh", flag: "\u{1F1E8}\u{1F1F3}", name: "\u4E2D\u6587" },
];

export function LanguageSelector() {
  const { locale, setLocale, t } = useI18n();
  const currentLang = LANG_OPTIONS.find((l) => l.value === locale) || LANG_OPTIONS[0];

  return (
    <Select value={locale} onValueChange={(v) => setLocale(v as "en" | "zh")}>
      <Tooltip>
        <TooltipTrigger render={<SelectTrigger size="sm" className="w-8 h-8 p-0 justify-center" aria-label={t("common.language")}>
            <SelectValue>
              <span className="text-sm">{currentLang.flag}</span>
            </SelectValue>
          </SelectTrigger>} />
        <TooltipContent>{t("common.language")}</TooltipContent>
      </Tooltip>
      <SelectContent>
        {LANG_OPTIONS.map((lang) => (
          <SelectItem key={lang.value} value={lang.value}>
            <span>{lang.flag}</span>
            <span>{lang.name}</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}