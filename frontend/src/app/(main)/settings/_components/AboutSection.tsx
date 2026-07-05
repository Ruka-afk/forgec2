import { SettingsData } from "./types";

export default function AboutSection({
  data, saving, onCheckUpdate,
}: {
  data: SettingsData;
  saving: boolean;
  onCheckUpdate: () => void;
}) {
  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-slate-700 to-slate-900 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-microchip text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">System Information</h2><p className="text-xs text-slate-400">Runtime and environment details</p></div>
        </div>
      </div>
      <div className="p-6">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
          {[
            { label: "Version", value: `v${data.ServerVersion ?? data.server_version ?? "2.0.0"}` },
            { label: "Uptime", value: data.Uptime ?? data.uptime ?? "-" },
            { label: "Go Version", value: data.GoVersion ?? data.go_version ?? "-" },
            { label: "Platform", value: `${data.GOOS ?? data.goos ?? "-"} / ${data.GOARCH ?? data.goarch ?? "-"}` },
            { label: "Goroutines", value: data.Goroutines ?? data.goroutines ?? "-" },
            { label: "Memory Alloc", value: data.AllocMem ? (data.AllocMem / 1024 / 1024).toFixed(1) + " MB" : "-" },
            { label: "Total Alloc", value: data.TotalAllocMem ? (data.TotalAllocMem / 1024 / 1024).toFixed(1) + " MB" : "-" },
            { label: "CPU Cores", value: data.NumCPU ?? data.num_cpu ?? "-" },
            { label: "Implants", value: `${data.TotalAgents ?? 0} (${data.OnlineAgents ?? data.online_agents ?? 0} online)` },
          ].map((stat, i) => (
            <div key={i} className={`bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4 border border-[var(--border)] ${stat.label === "Implants" ? "col-span-2" : ""}`}>
              <div className="text-xs text-slate-500 dark:text-slate-400">{stat.label}</div>
              <div className="font-medium text-slate-700 dark:text-slate-200 mt-1 font-mono text-xs">{stat.value}</div>
            </div>
          ))}
          <div className="bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4 border border-[var(--border)] col-span-2">
            <div className="text-xs text-slate-500 dark:text-slate-400">Data Directory</div>
            <div className="font-medium text-slate-700 dark:text-slate-200 mt-1 font-mono text-xs truncate">{data.DataDir ?? data.data_dir ?? "-"}</div>
          </div>
        </div>
        <div className="mt-6 flex flex-wrap gap-3">
          <button onClick={onCheckUpdate} className="h-10 px-4 bg-indigo-100 dark:bg-indigo-900/30 hover:bg-indigo-200 dark:hover:bg-indigo-800 text-indigo-700 dark:text-indigo-300 rounded-xl text-xs font-medium transition-colors">
            <i className="fa-solid fa-rotate mr-1"></i>Check for Updates
          </button>
        </div>
        <div className="mt-6 text-center text-xs text-slate-500 dark:text-slate-400">
          ForgeC2 v{data.ServerVersion ?? data.server_version ?? "2.0.0"} &bull; Multi-user mode<br />
          For authorized red team operations only
        </div>
      </div>
    </section>
  );
}
