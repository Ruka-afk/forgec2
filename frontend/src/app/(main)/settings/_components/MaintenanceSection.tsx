import { PurgeDays } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Wand2, Trash2 } from "lucide-react";

export default function MaintenanceSection({
  purgeDays, setPurgeDays, saving, onPurge, onPurgeScreenshots,
}: {
  purgeDays: PurgeDays;
  setPurgeDays: React.Dispatch<React.SetStateAction<PurgeDays>>;
  saving: boolean;
  onPurge: (type: string) => void;
  onPurgeScreenshots: () => void;
}) {
  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-orange-600 to-orange-700 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Wand2 className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-white">Data Maintenance</h2><p className="text-xs text-orange-200">Clean old data, free up space</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5 space-y-4">
        {[
          { label: "Purge Old Tasks", desc: "Delete completed/failed tasks older than specified days", type: "tasks" },
          { label: "Purge Audit Logs", desc: "Delete audit logs older than specified days", type: "audit" },
        ].map((item) => (
          <div key={item.type} className="p-4 bg-muted rounded-xl border border-border flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <div className="text-sm font-medium text-muted-foreground">{item.label}</div>
              <div className="text-xs text-muted-foreground mt-0.5">{item.desc}</div>
            </div>
            <div className="flex items-center gap-2">
              <Select value={purgeDays[item.type as keyof PurgeDays]} onValueChange={(v) => v && setPurgeDays({ ...purgeDays, [item.type]: v })}>
                <SelectTrigger className="w-[120px]"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="7">7 days</SelectItem>
                  <SelectItem value="14">14 days</SelectItem>
                  <SelectItem value="30">30 days</SelectItem>
                  <SelectItem value="60">60 days</SelectItem>
                  <SelectItem value="90">90 days</SelectItem>
                </SelectContent>
              </Select>
              <Button onClick={() => onPurge(item.type)} disabled={saving} className="px-4 h-8 bg-destructive/10 hover:bg-destructive/20 text-destructive rounded-xl text-xs font-medium transition-colors disabled:opacity-50">
                <Trash2 className="w-4 h-4" />Purge
              </Button>
            </div>
          </div>
        ))}
        <div className="p-4 bg-muted rounded-xl border border-border flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div className="text-sm font-medium text-muted-foreground">Purge Screenshots</div>
            <div className="text-xs text-muted-foreground mt-0.5">Delete all stored screenshots</div>
          </div>
          <Button onClick={onPurgeScreenshots} disabled={saving} className="px-4 h-8 bg-destructive/10 hover:bg-destructive/20 text-destructive rounded-xl text-xs font-medium transition-colors disabled:opacity-50">
            <Trash2 className="w-4 h-4" />Purge
          </Button>
        </div>
      </div>
    </Card>
  );
}
