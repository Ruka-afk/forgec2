import { PurgeDays } from "./types";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
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
      <CardHeaderRow icon={Wand2} tone="warning" title={t("settings.maintenance.title")} description={t("settings.maintenance.subtitle")} />
      <div className="p-(--card-spacing) space-y-4">
        {[
          { labelKey: "settings.maintenance.purge_tasks", descKey: "settings.maintenance.purge_tasks_desc", type: "tasks" },
          { labelKey: "settings.maintenance.purge_audit", descKey: "settings.maintenance.purge_audit_desc", type: "audit" },
        ].map((item) => (
          <div key={item.type} className="p-4 bg-muted rounded-lg border border-border flex flex-col sm:flex-row sm:items-center justify-between gap-3">
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
              <Button onClick={() => onPurge(item.type)} disabled={saving} className="px-4 h-8 bg-destructive/10 hover:bg-destructive/20 text-destructive rounded-lg text-xs font-medium transition-colors disabled:opacity-50">
                <Trash2 className="size-4" />{t("settings.maintenance.purge")}
              </Button>
            </div>
          </div>
        ))}
        <div className="p-4 bg-muted rounded-lg border border-border flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div className="text-sm font-medium text-muted-foreground">{t("settings.maintenance.purge_screenshots")}</div>
            <div className="text-xs text-muted-foreground mt-0.5">{t("settings.maintenance.purge_screenshots_desc")}</div>
          </div>
          <Button onClick={onPurgeScreenshots} disabled={saving} className="px-4 h-8 bg-destructive/10 hover:bg-destructive/20 text-destructive rounded-lg text-xs font-medium transition-colors disabled:opacity-50">
            <Trash2 className="size-4" />{t("settings.maintenance.purge")}
          </Button>
        </div>
      </div>
    </Card>
  );
}
