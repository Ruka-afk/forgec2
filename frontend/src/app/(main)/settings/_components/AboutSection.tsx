import { SettingsData } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Cpu, RotateCw } from "lucide-react";

export default function AboutSection({
  data, onCheckUpdate,
}: {
  data: SettingsData;
  onCheckUpdate: () => void;
}) {
  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-muted/50 to-muted px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Cpu className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">System Information</h2><p className="text-xs text-muted-foreground">Runtime and environment details</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          {[
            { label: "Version", value: `v${data.server_version ?? "2.0.0"}` },
            { label: "Uptime", value: data.uptime ?? "-" },
            { label: "Go Version", value: data.go_version ?? "-" },
            { label: "Platform", value: `${data.goos ?? "-"} / ${data.goarch ?? "-"}` },
            { label: "Goroutines", value: data.goroutines ?? "-" },
            { label: "Memory Alloc", value: data.alloc_mem ? (data.alloc_mem / 1024 / 1024).toFixed(1) + " MB" : "-" },
            { label: "Total Alloc", value: data.total_alloc_mem ? (data.total_alloc_mem / 1024 / 1024).toFixed(1) + " MB" : "-" },
            { label: "CPU Cores", value: data.num_cpu ?? "-" },
            { label: "Implants", value: `${data.total_agents ?? 0} (${data.online_agents ?? 0} online)` },
          ].map((stat, i) => (
            <div key={i} className={`bg-muted rounded-xl p-4 border border-border ${stat.label === "Implants" ? "col-span-2" : ""}`}>
              <div className="text-xs text-muted-foreground">{stat.label}</div>
              <div className="font-medium text-foreground mt-1 font-mono text-xs">{stat.value}</div>
            </div>
          ))}
          <div className="bg-muted rounded-xl p-4 border border-border col-span-2">
            <div className="text-xs text-muted-foreground">Data Directory</div>
            <div className="font-medium text-foreground mt-1 font-mono text-xs truncate">{data.data_dir ?? "-"}</div>
          </div>
        </div>
        <div className="mt-6 flex flex-wrap gap-3">
          <Button onClick={onCheckUpdate} className="h-10 px-4 bg-indigo-100 dark:bg-indigo-900/30 hover:bg-indigo-200 dark:hover:bg-indigo-800 text-indigo-700 dark:text-indigo-300 rounded-xl text-xs font-medium transition-colors">
            <RotateCw className="w-4 h-4" />Check for Updates
          </Button>
        </div>
        <div className="mt-6 text-center text-xs text-muted-foreground">
          ForgeC2 v{data.server_version ?? "2.0.0"} &bull; Multi-user mode
          <span className="block mt-1">For authorized red team operations only</span>
        </div>
      </div>
    </Card>
  );
}

