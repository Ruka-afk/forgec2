
import ThemeSection from "./ThemeSection";
import LanguageSection from "./LanguageSection";

/** Appearance groups the two purely client-side preferences — theme/density and
 *  UI language — that used to be separate top-level tabs. Neither touches the
 *  server, so splitting them across the settings navigation gave them the
 *  visual weight of configurable subsystems while adding nothing. */
export default function AppearanceSection({
  theme, onApplyTheme, language, onSetLanguage,
}: {
  theme: string;
  onApplyTheme: (t: string) => void;
  language: string;
  onSetLanguage: (code: string) => void;
}) {
  return (
    <div className="space-y-4">
      <ThemeSection theme={theme} onApplyTheme={onApplyTheme} />
      <LanguageSection language={language} onSetLanguage={onSetLanguage} />
    </div>
  );
}
