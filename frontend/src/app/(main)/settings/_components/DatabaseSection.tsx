import { SettingsData } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Copy, Database, Download, Minimize2 } from "lucide-react";

export default function DatabaseSection({
  data, saving, onVacuum, onBackup, onDownloadDB,
}: {
  data: SettingsData;
  saving: boolean;
  onVacuum: () => void;
  onBackup: () => void;
  onDownloadDB: () => void;
}) {
  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-cyan-600 to-cyan-700 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Database className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-white">Database</h2><p className="text-xs text-cyan-200">Statistics and management</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3 mb-6">
          {[
            { label: "Size", value: data.database_size ? (data.database_size / 1024 / 1024).toFixed(1) + " MB" : "-" },
            { label: "Agents", value: data.total_agents ?? 0 },
            { label: "Listeners", value: data.total_listeners ?? 0 },
            { label: "Tasks", value: data.total_tasks ?? 0 },
            { label: "Credentials", value: data.total_credentials ?? 0 },
            { label: "Audit Logs", value: data.total_audits ?? 0 },
          ].map((stat) => (
            <div key={stat.label} className="bg-muted rounded-xl p-4 border border-border">
              <div className="text-xs text-muted-foreground">{stat.label}</div>
              <div className="font-semibold text-sm text-foreground mt-1">{stat.value}</div>
            </div>
          ))}
        </div>
        <div className="flex flex-wrap gap-3">
          <Button onClick={onVacuum} disabled={saving} className="px-4 h-10 bg-primary/10 hover:bg-primary/20 text-primary rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <Minimize2 className="w-4 h-4" />VACUUM
          </Button>
          <Button onClick={onBackup} disabled={saving} className="px-4 h-10 bg-accent hover:bg-accent/80 text-accent-foreground rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <Copy className="w-4 h-4" />Backup
          </Button>
          <Button onClick={onDownloadDB} className="px-4 h-10 bg-emerald-100 dark:bg-emerald-900/30 hover:bg-emerald-200 dark:hover:bg-emerald-800 text-emerald-700 dark:text-emerald-400 rounded-xl text-sm font-medium transition-colors">
            <Download className="w-4 h-4" />Download
          </Button>
        </div>
      </div>
    </Card>
  );
}

