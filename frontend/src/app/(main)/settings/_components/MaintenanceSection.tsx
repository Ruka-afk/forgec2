import { PurgeDays } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Wand2, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function MaintenanceSection({
  purgeDays, setPurgeDays, saving, onPurge, onPurgeScreenshots,
}: {
  purgeDays: PurgeDays;
  setPurgeDays: React.Dispatch<React.SetStateAction<PurgeDays>>;
  saving: boolean;
  onPurge: (type: string) => void;
  onPurgeScreenshots: () => void;
}) {
  const { t } = useI18n();
  return (
    <Card className="overflow-hidden">
      <div className="bg-orange-500/10 border-b border-orange-500/20 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Wand2 className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">{t("settings.maintenance.title")}</h2><p className="text-xs text-muted-foreground">{t("settings.maintenance.subtitle")}</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5 space-y-4">
        {[
          { labelKey: "settings.maintenance.purge_tasks", descKey: "settings.maintenance.purge_tasks_desc", type: "tasks" },
          { labelKey: "settings.maintenance.purge_audit", descKey: "settings.maintenance.purge_audit_desc", type: "audit" },
        ].map((item) => (
          <div key={item.type} className="p-4 bg-muted rounded-xl border border-border flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <div className="text-sm font-medium text-muted-foreground">{t(item.labelKey)}</div>
              <div className="text-xs text-muted-foreground mt-0.5">{t(item.descKey)}</div>
            </div>
            <div className="flex items-center gap-2">
              <Select value={purgeDays[item.type as keyof PurgeDays]} onValueChange={(v) => v && setPurgeDays({ ...purgeDays, [item.type]: v })}>
                <SelectTrigger className="w-[120px]"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="7">{t("settings.maintenance.days").replace("{n}", "7")}</SelectItem>
                  <SelectItem value="14">{t("settings.maintenance.days").replace("{n}", "14")}</SelectItem>
                  <SelectItem value="30">{t("settings.maintenance.days").replace("{n}", "30")}</SelectItem>
                  <SelectItem value="60">{t("settings.maintenance.days").replace("{n}", "60")}</SelectItem>
                  <SelectItem value="90">{t("settings.maintenance.days").replace("{n}", "90")}</SelectItem>
                </SelectContent>
              </Select>
              <Button onClick={() => onPurge(item.type)} disabled={saving} className="px-4 h-8 bg-destructive/10 hover:bg-destructive/20 text-destructive rounded-xl text-xs font-medium transition-colors disabled:opacity-50">
                <Trash2 className="w-4 h-4" />{t("settings.maintenance.purge")}
              </Button>
            </div>
          </div>
        ))}
        <div className="p-4 bg-muted rounded-xl border border-border flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div className="text-sm font-medium text-muted-foreground">{t("settings.maintenance.purge_screenshots")}</div>
            <div className="text-xs text-muted-foreground mt-0.5">{t("settings.maintenance.purge_screenshots_desc")}</div>
          </div>
          <Button onClick={onPurgeScreenshots} disabled={saving} className="px-4 h-8 bg-destructive/10 hover:bg-destructive/20 text-destructive rounded-xl text-xs font-medium transition-colors disabled:opacity-50">
            <Trash2 className="w-4 h-4" />{t("settings.maintenance.purge")}
          </Button>
        </div>
      </div>
    </Card>
  );
}
