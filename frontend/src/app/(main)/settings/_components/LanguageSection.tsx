import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { CheckCircle2, Info, Languages } from "lucide-react";
import { useI18n } from "@/lib/i18n";

const SUPPORTED_LANGS = [
  { code: "en", name: "English", native: "English", flag: "\uD83C\uDDFA\uD83C\uDDF8" },
  { code: "zh", name: "Chinese", native: "\u4E2D\u6587", flag: "\uD83C\uDDE8\uD83C\uDDF3" },
];

export default function LanguageSection({ language, onSetLanguage }: { language: string; onSetLanguage: (code: string) => void }) {
  const { t } = useI18n();
  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={Languages} tone="info" title={t("settings.language.title")} description={t("settings.language.subtitle")} />
      <div className="p-(--card-spacing)">
        <div className="grid max-w-2xl grid-cols-1 gap-3 sm:grid-cols-2">
          {SUPPORTED_LANGS.map((lang) => (
            <Button key={lang.code} onClick={() => onSetLanguage(lang.code)} variant="ghost"
              className={`relative h-auto min-h-24 items-center justify-start gap-3 whitespace-normal rounded-xl border p-4 text-left transition-colors ${language === lang.code ? "border-info/50 bg-info/10 shadow-xs" : "border-border bg-muted/45 hover:border-info/45 hover:bg-info/5"}`}>
              {language === lang.code && <CheckCircle2 className="absolute right-3 top-3 size-4 text-info" />}
              <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-card text-2xl shadow-xs">{lang.flag}</span>
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-foreground">{lang.native}</span>
                <span className="mt-1 block text-xs text-muted-foreground">{lang.name}</span>
              </span>
            </Button>
          ))}
        </div>
        <div className="mt-6 bg-muted rounded-lg p-4">
          <div className="flex items-center gap-3">
            <Info className="size-4" />
            <div className="text-xs text-muted-foreground">{t("settings.language.cookie_hint")}</div>
          </div>
        </div>
      </div>
    </Card>
  );
}
