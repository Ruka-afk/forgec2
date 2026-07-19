import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Monitor, Info, Moon, Palette, Sun } from "lucide-react";

export default function ThemeSection({ theme, onApplyTheme }: { theme: string; onApplyTheme: (t: string) => void }) {
  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-violet-600 to-indigo-600 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Palette className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-white">Theme</h2><p className="text-xs text-violet-200">Customize appearance</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <div className="mb-6">
          <h3 className="text-sm font-semibold text-foreground mb-4 flex items-center gap-2"><Moon className="w-4 h-4 text-muted-foreground" />Theme Mode</h3>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            {["light", "dark", "system"].map((mode) => (
              <Button key={mode} onClick={() => onApplyTheme(mode)} variant="ghost"
                className={`p-4 border rounded-xl transition-colors capitalize ${theme === mode ? "border-indigo-500 bg-indigo-50 dark:bg-indigo-900/30" : "bg-muted border-border hover:border-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-900/30"}`}>
                <div className={`text-2xl mb-2 ${mode === "light" ? "text-amber-500" : mode === "dark" ? "text-indigo-400" : "text-muted-foreground"}`}>
                  {mode === "light" ? <Sun className="w-6 h-6" /> : mode === "dark" ? <Moon className="w-6 h-6" /> : <Monitor className="w-6 h-6" />}
                </div>
                <div className="text-sm font-medium text-muted-foreground">{mode}</div>
                <div className="text-xs text-muted-foreground mt-1">{mode === "light" ? "Clean and bright" : mode === "dark" ? "Easy on the eyes" : "Match OS setting"}</div>
              </Button>
            ))}
          </div>
        </div>
        <div className="bg-muted rounded-xl p-4">
          <div className="flex items-center gap-3">
            <Info className="w-4 h-4" />
            <div className="text-xs text-muted-foreground">Theme settings are saved to localStorage.</div>
          </div>
        </div>
      </div>
    </Card>
  );
}
