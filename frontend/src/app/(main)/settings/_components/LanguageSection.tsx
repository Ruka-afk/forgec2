import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Info, Languages } from "lucide-react";
import { useI18n } from "@/lib/i18n";

const SUPPORTED_LANGS = [
  { code: "en", name: "English", native: "English", flag: "\uD83C\uDDFA\uD83C\uDDF8" },
  { code: "zh", name: "Chinese", native: "\u4E2D\u6587", flag: "\uD83C\uDDE8\uD83C\uDDF3" },
];

export default function LanguageSection({ language, onSetLanguage }: { language: string; onSetLanguage: (code: string) => void }) {
  const { t } = useI18n();
  return (
    <Card className="overflow-hidden">
      <div className="bg-info/10 border-b border-info/20 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Languages className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">{t("settings.language.title")}</h2><p className="text-xs text-muted-foreground">{t("settings.language.subtitle")}</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
          {SUPPORTED_LANGS.map((lang) => (
            <Button key={lang.code} onClick={() => onSetLanguage(lang.code)} variant="ghost"
              className={`p-4 border rounded-xl transition-colors ${language === lang.code ? "border-info/50 bg-info/15" : "bg-muted border-border hover:border-info hover:bg-info/15"}`}>
              <div className="text-2xl mb-2">{lang.flag}</div>
              <div className="text-sm font-medium text-muted-foreground">{lang.native}</div>
              <div className="text-xs text-muted-foreground mt-1">{lang.name}</div>
            </Button>
          ))}
        </div>
        <div className="mt-6 bg-muted rounded-xl p-4">
          <div className="flex items-center gap-3">
            <Info className="w-4 h-4" />
            <div className="text-xs text-muted-foreground">{t("settings.language.cookie_hint")}</div>
          </div>
        </div>
      </div>
    </Card>
  );
}
