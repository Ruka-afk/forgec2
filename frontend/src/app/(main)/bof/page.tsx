"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal } from "@/components/UI";

interface BOFFile {
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  Size?: number | string;
  size?: number;
  Description?: string;
  description?: string;
  Architecture?: string;
  architecture?: string;
  CreatedBy?: string;
  created_by?: string;
  CreatedAt?: string;
  created_at?: string;
}

interface Execution {
  ID?: string;
  id?: string;
  BofName?: string;
  bof_name?: string;
  AgentHostname?: string;
  agent_hostname?: string;
  Status?: string;
  status?: string;
  Result?: string;
  result?: string;
  Args?: string;
  args?: string;
  CreatedAt?: string;
  created_at?: string;
}

interface QuickBOF {
  name: string;
  desc: string;
  arch: string;
  args: string;
}

const quickBOFLibrary: QuickBOF[] = [
  { name: "adcs_enum", desc: "Enumerate AD CS templates and certificate authorities", arch: "x64", args: "" },
  { name: "sc_shutdown_elevated", desc: "Shutdown system with elevated privileges", arch: "x64", args: "" },
  { name: "netuserenum", desc: "Enumerate domain users via various methods", arch: "x64", args: "/groups" },
  { name: "enumerate-laps", desc: "Enumerate LAPS passwords from AD", arch: "x64", args: "" },
  { name: "uptime", desc: "Get system uptime information", arch: "x64", args: "" },
  { name: "env-list", desc: "List environment variables", arch: "x64", args: "" },
  { name: "ldap-search", desc: "Perform LDAP searches from beacon", arch: "x64", args: "(objectClass=*)" },
  { name: "kerberoast", desc: "Request TGS tickets for kerberoasting", arch: "x64", args: "" },
  { name: "clipboard", desc: "Monitor clipboard contents", arch: "x64", args: "" },
  { name: "wts_enum", desc: "Enumerate Remote Desktop sessions", arch: "x64", args: "" },
  { name: "window-list", desc: "List visible windows on desktop", arch: "x64", args: "" },
  { name: "tcp-scan", desc: "Internal TCP port scanner", arch: "x64", args: "10.0.0.1 80-443" },
];

interface RepoItem {
  ID?: string; id?: string;
  Name?: string; name?: string;
  Description?: string; description?: string;
  URL?: string; url?: string;
  Author?: string; author?: string;
  Stars?: number; stars?: number;
  Downloads?: number; downloads?: number;
  Category?: string; category?: string;
  Architecture?: string; architecture?: string;
  Rating?: number; rating?: number;
  Reviews?: number; reviews?: number;
  Imported?: boolean; imported?: boolean;
}

interface ImportStatus {
  loading: boolean;
  message: string;
  success: boolean;
}

export default function BOFPage() {
  const [bofFiles, setBOFFiles] = useState<BOFFile[]>([]);
  const [executions, setExecutions] = useState<Execution[]>([]);
  const [loading, setLoading] = useState(true);
  const [showUpload, setShowUpload] = useState(false);
  const [showRun, setShowRun] = useState(false);
  const [showInfo, setShowInfo] = useState<BOFFile | null>(null);
  const [editTarget, setEditTarget] = useState<BOFFile | null>(null);
  const [uploadName, setUploadName] = useState("");
  const [uploadDesc, setUploadDesc] = useState("");
  const [uploadArch, setUploadArch] = useState("x64");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [runBofId, setRunBofId] = useState("");
  const [runAgent, setRunAgent] = useState("");
  const [runArgs, setRunArgs] = useState("");
  const [agents, setAgents] = useState<Array<{ id: string; hostname: string }>>([]);
  const [activeTab, setActiveTab] = useState<"bof" | "exec" | "quick" | "repo">("bof");
  const [toastMsg, setToastMsg] = useState<string | null>(null);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [repoItems, setRepoItems] = useState<RepoItem[]>([]);
  const [importUrl, setImportUrl] = useState("");
  const [importName, setImportName] = useState("");
  const [importStatus, setImportStatus] = useState<ImportStatus | null>(null);
  const [repoSearch, setRepoSearch] = useState("");
  const [filterCategory, setFilterCategory] = useState("all");
  const [filterArch, setFilterArch] = useState("all");
  const [sortBy, setSortBy] = useState<"stars" | "name">("stars");

  useEffect(() => { if (toastMsg) { const t = setTimeout(() => setToastMsg(null), 3000); return () => clearTimeout(t); } }, [toastMsg]);

  const loadBOF = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/bof&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setBOFFiles(data.BOFFiles || data.bofs || data.files || []);
      const execRes = await fetch(`${API_BASE}?p=/api/bof/results&format=json`);
      if (!execRes.ok) throw new Error(`HTTP ${execRes.status}`);
      const execData = await execRes.json();
      setExecutions(execData.results || execData.Results || []);
      const agentRes = await fetch(`${API_BASE}?p=/agents&format=json`);
      if (!agentRes.ok) throw new Error(`HTTP ${agentRes.status}`);
      const agentData = await agentRes.json();
      setAgents((agentData.Agents || agentData.agents || []).map((a: Record<string, unknown>) => ({ id: String(a.ID || a.id || ""), hostname: String(a.Hostname || a.hostname || "") })));
    } catch (e) { console.error("BOF: load data failed", e); } finally { setLoading(false); }
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadBOF()); }, [loadBOF]);

  const loadRepo = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/bof_repo&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setRepoItems(data.data || data.repo || data.items || []);
    } catch { setRepoItems([]); }
  }, []);

  useEffect(() => { if (activeTab === "repo") loadRepo(); }, [activeTab, loadRepo]);

  const handleImport = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!importUrl.trim()) return;
    setImportStatus({ loading: true, message: "Importing BOF from URL...", success: false });
    try {
      const res = await fetch(`${API_BASE}?p=/api/bof/repos/import&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: importUrl, name: importName || undefined }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setImportStatus({ loading: false, message: data.message || "Import completed successfully", success: true });
      setImportUrl("");
      setImportName("");
      loadRepo();
      loadBOF();
    } catch {
      setImportStatus({ loading: false, message: "Import failed - check URL and try again", success: false });
    }
  };

  const handleImportFromRepo = async (item: RepoItem) => {
    setImportStatus({ loading: true, message: `Importing ${item.Name || item.name}...`, success: false });
    try {
      await fetch(`${API_BASE}?p=/api/bof/repos/import&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: item.URL || item.url, name: item.Name || item.name }),
      });
      setImportStatus({ loading: false, message: `${item.Name || item.name} imported successfully`, success: true });
      loadRepo();
      loadBOF();
    } catch {
      setImportStatus({ loading: false, message: "Import failed", success: false });
    }
  };

  const handleRate = async (itemId: string, rating: number) => {
    try {
      await fetch(`${API_BASE}?p=/api/bof/repos/${itemId}/rate&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ rating }),
      });
      loadRepo();
    } catch (e) { console.error("BOF Repo: rating failed", e); }
  };

  const renderStars = (rating: number | undefined, itemId: string, interactive: boolean = false) => {
    const r = rating ?? 0;
    return (
      <div className="flex items-center gap-0.5">
        {[1, 2, 3, 4, 5].map((star) => (
          <button key={star} onClick={() => interactive && handleRate(itemId, star)}
            className={`${interactive ? "cursor-pointer hover:scale-110 transition-transform" : "cursor-default"} text-xs ${star <= r ? "text-amber-500" : "text-slate-300 dark:text-slate-600"}`}>
            <i className="fa-solid fa-star"></i>
          </button>
        ))}
      </div>
    );
  };

  const handleUpload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!uploadFile) return;
    const formData = new FormData();
    formData.append("file", uploadFile);
    formData.append("name", uploadName);
    formData.append("description", uploadDesc);
    formData.append("architecture", uploadArch);
    try {
      await fetch(`${API_BASE}?p=/api/bof/upload&format=json`, { method: "POST", body: formData });
    } catch (e) { console.error("BOF: upload failed", e); }
    setShowUpload(false);
    setUploadFile(null);
    setUploadName("");
    setUploadDesc("");
    setUploadArch("x64");
    loadBOF();
  };

  const handleRun = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const body = new URLSearchParams({ agent_id: runAgent, args: runArgs }).toString();
      await fetch(`${API_BASE}?p=/api/bof/${runBofId}/run&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body,
      });
    } catch (e) { console.error("BOF: run failed", e); }
    setShowRun(false);
    loadBOF();
  };

  const handleEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editTarget) return;
    try {
      const body = new URLSearchParams({ name: String(editTarget.Name || editTarget.name || ""), description: String(editTarget.Description || editTarget.description || "") }).toString();
      await fetch(`${API_BASE}?p=/api/bof/${editTarget.ID || editTarget.id}/edit&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body,
      });
    } catch (e) { console.error("BOF: edit failed", e); }
    setEditTarget(null);
    loadBOF();
  };

  const handleDelete = (id: string) => {
    setCfm({msg: "Delete this BOF?", cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/api/bof/${id}&format=json`, { method: "DELETE" });
      } catch (e) { console.error("BOF: delete failed", e); }
      loadBOF();
    }});
  };

  const handleQuickRun = (bof: QuickBOF) => {
    const bofFile = bofFiles.find(f => (f.Name || f.name || "").toLowerCase() === bof.name.toLowerCase());
    if (bofFile) {
      setRunBofId(bofFile.ID || bofFile.id || "");
      setRunAgent(agents[0]?.id || "");
      setRunArgs(bof.args);
      setShowRun(true);
    } else {
      setToastMsg(`BOF "${bof.name}" not uploaded. Import from BOF Repo first.`);
    }
  };

  const formatBytes = (bytes: number | string | undefined) => {
    if (!bytes) return "0 B";
    const b = Number(bytes);
    if (b < 1024) return `${b} B`;
    if (b < 1048576) return `${(b / 1024).toFixed(1)} KB`;
    return `${(b / 1048576).toFixed(1)} MB`;
  };

  const getStatusColor = (status: string) => {
    const s = status?.toLowerCase() ?? "";
    if (s === "success" || s === "completed") return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400";
    if (s === "failed") return "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400";
    if (s === "running") return "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400";
    return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400";
  };

  if (loading) return <div className="text-gray-500 py-8 text-center"><i className="fa-solid fa-circle-notch fa-spin text-2xl"></i></div>;

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="mb-6">
        <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">BOF Management</h1>
        <p className="text-slate-500 dark:text-slate-400 text-xs sm:text-sm mt-1">Beacon Object Files - upload, execute, and manage BOF tools</p>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-4 gap-4 mb-6">
        <div className="ui-card p-4 flex items-center gap-3">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/20 rounded-2xl flex items-center justify-center">
            <i className="fa-solid fa-cube text-indigo-600 dark:text-indigo-400"></i>
          </div>
          <div>
            <div className="text-xl font-bold text-slate-900 dark:text-slate-100">{bofFiles.length}</div>
            <div className="text-xs text-slate-500">Uploaded BOFs</div>
          </div>
        </div>
        <div className="ui-card p-4 flex items-center gap-3">
          <div className="w-10 h-10 bg-emerald-100 dark:bg-emerald-900/20 rounded-2xl flex items-center justify-center">
            <i className="fa-solid fa-check text-emerald-600 dark:text-emerald-400"></i>
          </div>
          <div>
            <div className="text-xl font-bold text-slate-900 dark:text-slate-100">{executions.length}</div>
            <div className="text-xs text-slate-500">Executions</div>
          </div>
        </div>
        <div className="ui-card p-4 flex items-center gap-3">
          <div className="w-10 h-10 bg-amber-100 dark:bg-amber-900/20 rounded-2xl flex items-center justify-center">
            <i className="fa-solid fa-chart-pie text-amber-600 dark:text-amber-400"></i>
          </div>
          <div>
            <div className="text-xl font-bold text-slate-900 dark:text-slate-100">
              {executions.length > 0 ? `${Math.round((executions.filter((e) => (e.Status ?? e.status) === "success").length / executions.length) * 100)}%` : "N/A"}
            </div>
            <div className="text-xs text-slate-500">Success Rate</div>
          </div>
        </div>
        <div className="ui-card p-4 flex items-center gap-3">
          <div className="w-10 h-10 bg-purple-100 dark:bg-purple-900/20 rounded-2xl flex items-center justify-center">
            <i className="fa-solid fa-book text-purple-600 dark:text-purple-400"></i>
          </div>
          <div>
            <div className="text-xl font-bold text-slate-900 dark:text-slate-100">{agents.length}</div>
            <div className="text-xs text-slate-500">Available Agents</div>
          </div>
        </div>
      </div>

      <div className="flex gap-1 mb-6 ui-card p-1.5">
        <button onClick={() => setActiveTab("bof")}
          className={`px-4 py-2 text-sm font-medium rounded-2xl transition-colors ${activeTab === "bof" ? "bg-indigo-50 text-indigo-600 dark:bg-indigo-900/20 dark:text-indigo-400" : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"}`}>
          <i className="fa-solid fa-cube mr-1.5"></i>BOF Library
        </button>
        <button onClick={() => setActiveTab("exec")}
          className={`px-4 py-2 text-sm font-medium rounded-2xl transition-colors ${activeTab === "exec" ? "bg-indigo-50 text-indigo-600 dark:bg-indigo-900/20 dark:text-indigo-400" : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"}`}>
          <i className="fa-solid fa-terminal mr-1.5"></i>Executions
        </button>
        <button onClick={() => setActiveTab("quick")}
          className={`px-4 py-2 text-sm font-medium rounded-2xl transition-colors ${activeTab === "quick" ? "bg-indigo-50 text-indigo-600 dark:bg-indigo-900/20 dark:text-indigo-400" : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"}`}>
          <i className="fa-solid fa-bolt mr-1.5"></i>Quick BOF
        </button>
        <button onClick={() => setActiveTab("repo")}
          className={`px-4 py-2 text-sm font-medium rounded-2xl transition-colors ${activeTab === "repo" ? "bg-indigo-50 text-indigo-600 dark:bg-indigo-900/20 dark:text-indigo-400" : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"}`}>
          <i className="fa-solid fa-book mr-1.5"></i>Repository
        </button>
      </div>

      <div className="flex items-center justify-between mb-4">
        <div></div>
        {activeTab === "bof" || activeTab === "quick" ? (
        <button onClick={() => setShowUpload(true)}
          className="px-4 h-10 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl text-sm font-medium transition-colors flex items-center gap-2">
          <i className="fa-solid fa-upload"></i>
          Upload BOF
        </button>
        ) : null}
      </div>

      {activeTab === "bof" && (
        <div className="ui-card overflow-hidden">
          {bofFiles.length > 0 ? (
            <div className="divide-y divide-slate-100 dark:divide-slate-700">
              {bofFiles.map((b, i) => {
                const bid = b.ID || b.id || String(i);
                const bname = b.Name || b.name || "Unknown";
                const bdesc = b.Description || b.description || "";
                const bsize = formatBytes(b.Size ?? b.size);
                const barch = b.Architecture || b.architecture || "x64";
                return (
                  <div key={bid} className="px-5 py-4 hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3 flex-1 min-w-0">
                        <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/20 rounded-lg flex items-center justify-center text-indigo-600 dark:text-indigo-400 text-xs font-bold flex-shrink-0">
                          <i className="fa-solid fa-cube"></i>
                        </div>
                        <div className="min-w-0">
                          <div className="text-sm font-medium text-slate-900 dark:text-slate-100 truncate">{bname}</div>
                          <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                            {bsize} · {barch} {bdesc ? `· ${bdesc}` : ""}
                          </div>
                        </div>
                      </div>
                      <div className="flex gap-2 ml-4">
                        <button onClick={() => { setRunBofId(bid); setRunAgent(agents[0]?.id || ""); setShowRun(true); }}
                          className="px-3 py-1.5 text-xs bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-400 rounded-lg border border-emerald-200 dark:border-emerald-700 hover:bg-emerald-100 dark:hover:bg-emerald-900/40 transition-colors">
                          <i className="fa-solid fa-play mr-1"></i>Run
                        </button>
                        <button onClick={() => setShowInfo(b)}
                          className="px-3 py-1.5 text-xs bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300 rounded-lg border border-[var(--border)] hover:bg-slate-200 dark:hover:bg-slate-600 transition-colors">
                          <i className="fa-solid fa-info"></i>
                        </button>
                        <button onClick={() => setEditTarget(b)}
                          className="px-3 py-1.5 text-xs bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300 rounded-lg border border-[var(--border)] hover:bg-slate-200 dark:hover:bg-slate-600 transition-colors">
                          <i className="fa-solid fa-pen"></i>
                        </button>
                        <button onClick={() => handleDelete(bid)}
                          className="px-3 py-1.5 text-xs bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400 rounded-lg border border-red-200 dark:border-red-700 hover:bg-red-100 dark:hover:bg-red-900/40 transition-colors">
                          <i className="fa-solid fa-trash"></i>
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="text-center py-12 text-slate-400 dark:text-slate-500">
              <i className="fa-solid fa-cube text-3xl mb-2 text-slate-300 dark:text-slate-600"></i>
              <p>No BOF files uploaded</p>
              <p className="text-xs mt-1">Upload from local or import from BOF Repo</p>
            </div>
          )}
        </div>
      )}

      {activeTab === "exec" && (
        <div className="ui-card overflow-hidden">
          {executions.length > 0 ? (
            <div className="divide-y divide-slate-100 dark:divide-slate-700">
              {executions.map((ex, i) => (
                <div key={ex.ID || ex.id || i} className="px-5 py-4">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 bg-emerald-100 dark:bg-emerald-900/20 rounded-lg flex items-center justify-center text-emerald-600 dark:text-emerald-400">
                        <i className="fa-solid fa-terminal text-xs"></i>
                      </div>
                      <div>
                        <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{ex.BofName || ex.bof_name || "Unknown"}</div>
                        <div className="text-xs text-slate-500">{ex.AgentHostname || ex.agent_hostname || "Unknown"} {ex.Args || ex.args ? `· args: ${ex.Args || ex.args}` : ""}</div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className={`text-[10px] px-2 py-0.5 rounded-full ${getStatusColor(ex.Status || ex.status || "")}`}>{ex.Status || ex.status || "pending"}</span>
                      <span className="text-xs text-slate-500">{ex.CreatedAt || ex.created_at || ""}</span>
                    </div>
                  </div>
                  {(ex.Result || ex.result) && (
                    <pre className="text-xs font-mono text-emerald-300 bg-slate-900 rounded-lg p-3 mt-2 max-h-40 overflow-y-auto whitespace-pre-wrap border border-slate-700">{ex.Result || ex.result}</pre>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-12 text-slate-400 dark:text-slate-500">
              <i className="fa-solid fa-terminal text-3xl mb-2 text-slate-300 dark:text-slate-600"></i>
              <p>No executions yet</p>
            </div>
          )}
        </div>
      )}

      {activeTab === "quick" && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {quickBOFLibrary.map((bof) => {
            const isUploaded = bofFiles.some(f => (f.Name || f.name || "").toLowerCase() === bof.name.toLowerCase());
            return (
              <div key={bof.name} className="ui-card p-4 hover:shadow-md transition-shadow">
                <div className="flex items-center justify-between mb-2">
                  <div className="text-sm font-medium text-slate-900 dark:text-slate-100 font-mono">{bof.name}</div>
                  <span className={`text-[10px] px-2 py-0.5 rounded-full ${isUploaded ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : "bg-slate-100 text-slate-500 dark:bg-slate-700 dark:text-slate-400"}`}>
                    {isUploaded ? "Ready" : "Not Installed"}
                  </span>
                </div>
                <div className="text-xs text-slate-500 dark:text-slate-400 mb-1">{bof.desc}</div>
                <div className="flex items-center justify-between mt-3">
                  <span className="text-[10px] px-2 py-0.5 rounded bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300 font-mono">{bof.arch}</span>
                  <button onClick={() => handleQuickRun(bof)} disabled={!isUploaded}
                    className="px-3 py-1.5 text-xs bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg transition-colors">
                    <i className="fa-solid fa-bolt mr-1"></i>Quick Run
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {activeTab === "repo" && (
        <div>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
            <div className="ui-card p-4 flex items-center gap-3">
              <div className="w-10 h-10 bg-purple-100 dark:bg-purple-900/20 rounded-2xl flex items-center justify-center">
                <i className="fa-solid fa-layer-group text-purple-600 dark:text-purple-400"></i>
              </div>
              <div>
                <div className="text-xl font-bold text-slate-900 dark:text-slate-100">{repoItems.length}</div>
                <div className="text-xs text-slate-500">Community BOFs</div>
              </div>
            </div>
            <div className="ui-card p-4 flex items-center gap-3">
              <div className="w-10 h-10 bg-emerald-100 dark:bg-emerald-900/20 rounded-2xl flex items-center justify-center">
                <i className="fa-solid fa-download text-emerald-600 dark:text-emerald-400"></i>
              </div>
              <div>
                <div className="text-xl font-bold text-slate-900 dark:text-slate-100">{repoItems.filter(i => i.Imported ?? i.imported).length}</div>
                <div className="text-xs text-slate-500">Imported</div>
              </div>
            </div>
            <div className="ui-card p-4 flex items-center gap-3">
              <div className="w-10 h-10 bg-amber-100 dark:bg-amber-900/20 rounded-2xl flex items-center justify-center">
                <i className="fa-solid fa-star text-amber-600 dark:text-amber-400"></i>
              </div>
              <div>
                <div className="text-xl font-bold text-slate-900 dark:text-slate-100">{repoItems.filter(i => (i.Rating ?? i.rating ?? 0) >= 4).length}</div>
                <div className="text-xs text-slate-500">Highly Rated</div>
              </div>
            </div>
          </div>

          <div className="ui-card p-5 mb-6">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/20 rounded-lg flex items-center justify-center text-indigo-600 dark:text-indigo-400">
                <i className="fa-solid fa-link"></i>
              </div>
              <span className="text-sm font-semibold text-slate-900 dark:text-slate-100">Import from URL</span>
            </div>
            <form onSubmit={handleImport} className="flex gap-3">
              <input type="text" placeholder="https://raw.githubusercontent.com/.../example.o" value={importUrl} onChange={(e) => setImportUrl(e.target.value)}
                className="flex-1 bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100" />
              <input type="text" placeholder="BOF Name (optional)" value={importName} onChange={(e) => setImportName(e.target.value)}
                className="w-52 bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100" />
              <button type="submit"
                className="px-5 h-10 bg-indigo-600 hover:bg-indigo-700 rounded-2xl text-sm text-white font-medium transition-colors">
                <i className="fa-solid fa-download mr-1.5"></i>Import
              </button>
            </form>
            {importStatus && (
              <div className={`mt-3 p-3 rounded-2xl text-xs flex items-center gap-2 ${
                importStatus.loading ? "bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-400" :
                importStatus.success ? "bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-400" :
                "bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400"
              }`}>
                <i className={`fa-solid ${importStatus.loading ? "fa-circle-notch fa-spin" : importStatus.success ? "fa-check-circle" : "fa-exclamation-circle"}`}></i>
                {importStatus.message}
              </div>
            )}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Search</label>
              <input type="text" placeholder="Search BOFs..." value={repoSearch} onChange={e => setRepoSearch(e.target.value)}
                className="w-full ui-card px-4 h-9 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Category</label>
              <select value={filterCategory} onChange={e => setFilterCategory(e.target.value)}
                className="w-full ui-card px-4 h-9 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                <option value="all">All Categories</option>
                {Array.from(new Set(repoItems.map(i => i.Category || i.category || "Uncategorized"))).map(c => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Architecture</label>
              <select value={filterArch} onChange={e => setFilterArch(e.target.value)}
                className="w-full ui-card px-4 h-9 text-sm focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                <option value="all">All Architectures</option>
                {Array.from(new Set(repoItems.map(i => i.Architecture || i.architecture || "x64"))).map(a => <option key={a} value={a}>{a}</option>)}
              </select>
            </div>
          </div>

          <div className="flex items-center justify-between mb-4">
            <span className="text-sm text-slate-500">
              {(() => {
                const filtered = repoItems.filter(item => {
                  const name = (item.Name || item.name || "").toLowerCase();
                  const desc = (item.Description || item.description || "").toLowerCase();
                  const author = (item.Author || item.author || "").toLowerCase();
                  const query = repoSearch.toLowerCase();
                  const matchSearch = !query || name.includes(query) || desc.includes(query) || author.includes(query);
                  const matchCategory = filterCategory === "all" || (item.Category || item.category) === filterCategory;
                  const matchArch = filterArch === "all" || (item.Architecture || item.architecture) === filterArch;
                  return matchSearch && matchCategory && matchArch;
                }).sort((a, b) => {
                  if (sortBy === "stars") return (b.Stars ?? b.stars ?? 0) - (a.Stars ?? a.stars ?? 0);
                  if (sortBy === "name") return (a.Name ?? a.name ?? "").localeCompare(b.Name ?? b.name ?? "");
                  return 0;
                });
                return `${filtered.length} BOFs found`;
              })()}
            </span>
            <div className="flex items-center gap-2">
              <span className="text-xs text-slate-500">Sort by:</span>
              <select value={sortBy} onChange={e => setSortBy(e.target.value as "stars" | "name")}
                className="ui-card rounded-lg px-3 h-8 text-xs focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                <option value="stars">Popularity</option>
                <option value="name">Name</option>
              </select>
            </div>
          </div>

          {repoItems.length === 0 ? (
            <div className="text-center py-12 text-slate-400 dark:text-slate-500">
              <i className="fa-solid fa-circle-notch fa-spin text-3xl text-indigo-500"></i>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {repoItems.filter(item => {
                const name = (item.Name || item.name || "").toLowerCase();
                const desc = (item.Description || item.description || "").toLowerCase();
                const author = (item.Author || item.author || "").toLowerCase();
                const query = repoSearch.toLowerCase();
                const matchSearch = !query || name.includes(query) || desc.includes(query) || author.includes(query);
                const matchCategory = filterCategory === "all" || (item.Category || item.category) === filterCategory;
                const matchArch = filterArch === "all" || (item.Architecture || item.architecture) === filterArch;
                return matchSearch && matchCategory && matchArch;
              }).sort((a, b) => {
                if (sortBy === "stars") return (b.Stars ?? b.stars ?? 0) - (a.Stars ?? a.stars ?? 0);
                if (sortBy === "name") return (a.Name ?? a.name ?? "").localeCompare(b.Name ?? b.name ?? "");
                return 0;
              }).map((item, i) => {
                const itemId = item.ID || item.id || String(i);
                const imported = item.Imported || item.imported;
                return (
                  <div key={itemId} className="ui-card p-5 hover:shadow-md transition-shadow">
                    <div className="flex items-start justify-between mb-3">
                      <div className="flex items-center gap-3">
                        <div className={`w-10 h-10 rounded-2xl flex items-center justify-center ${imported ? "bg-emerald-100 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400" : "bg-purple-100 dark:bg-purple-900/20 text-purple-600 dark:text-purple-400"}`}>
                          <i className="fa-solid fa-cube"></i>
                        </div>
                        <div>
                          <div className="text-sm font-semibold text-slate-900 dark:text-slate-100 font-mono">{item.Name || item.name || "Unnamed"}</div>
                          <div className="text-xs text-slate-500 dark:text-slate-400">
                            by {item.Author || item.author || "Unknown"}
                            {item.Category || item.category ? ` · ${item.Category || item.category}` : ""}
                          </div>
                        </div>
                      </div>
                      <span className={`text-[10px] px-2 py-0.5 rounded-full ${imported ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : "bg-slate-100 text-slate-500 dark:bg-slate-700 dark:text-slate-400"}`}>
                        {imported ? "Imported" : "Available"}
                      </span>
                    </div>
                    <p className="text-xs text-slate-600 dark:text-slate-400 mb-3 line-clamp-2">{item.Description || item.description || "No description"}</p>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-4 text-xs text-slate-500">
                        {renderStars(item.Rating ?? item.rating, itemId, true)}
                        <span className="flex items-center gap-1"><i className="fa-regular fa-star"></i>{item.Reviews ?? item.reviews ?? 0}</span>
                        <span className="flex items-center gap-1"><i className="fa-solid fa-download"></i>{item.Downloads ?? item.downloads ?? 0}</span>
                        <span className="px-2 py-0.5 rounded bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300 font-mono">{item.Architecture || item.architecture || "x64"}</span>
                      </div>
                      <button onClick={() => handleImportFromRepo(item)} disabled={imported}
                        className="px-3 py-1.5 text-xs bg-indigo-600 hover:bg-indigo-700 disabled:bg-slate-300 dark:disabled:bg-slate-600 disabled:cursor-not-allowed text-white rounded-lg transition-colors">
                        <i className={`fa-solid ${imported ? "fa-check" : "fa-download"} mr-1`}></i>
                        {imported ? "Imported" : "Import"}
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {showUpload && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowUpload(false)}>
          <div className="ui-card shadow-xl w-full max-w-md p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center gap-3 mb-5">
              <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/20 rounded-lg flex items-center justify-center text-indigo-600 dark:text-indigo-400">
                <i className="fa-solid fa-upload"></i>
              </div>
              <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">Upload BOF</div>
            </div>
            <form onSubmit={handleUpload} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">BOF File (.o)</label>
                <input type="file" accept=".o" onChange={(e) => setUploadFile(e.target.files?.[0] || null)} required
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm file:mr-3 file:py-1 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-indigo-50 file:text-indigo-600 dark:file:bg-indigo-900/20 dark:file:text-indigo-400" />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Name</label>
                <input type="text" placeholder="BOF name" value={uploadName} onChange={(e) => setUploadName(e.target.value)}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm" />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Description</label>
                <input type="text" placeholder="Brief description" value={uploadDesc} onChange={(e) => setUploadDesc(e.target.value)}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm" />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Architecture</label>
                <select value={uploadArch} onChange={(e) => setUploadArch(e.target.value)}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm">
                  <option value="x64">x64</option>
                  <option value="x86">x86</option>
                </select>
              </div>
              <button type="submit" className="w-full h-10 bg-indigo-600 hover:bg-indigo-700 rounded-2xl text-sm text-white font-medium transition-colors">Upload</button>
            </form>
          </div>
        </div>
      )}

      {showRun && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowRun(false)}>
          <div className="ui-card shadow-xl w-full max-w-md p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center gap-3 mb-5">
              <div className="w-8 h-8 bg-emerald-100 dark:bg-emerald-900/20 rounded-lg flex items-center justify-center text-emerald-600 dark:text-emerald-400">
                <i className="fa-solid fa-play"></i>
              </div>
              <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">Execute BOF</div>
            </div>
            <form onSubmit={handleRun} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Agent</label>
                <select value={runAgent} onChange={(e) => setRunAgent(e.target.value)}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm" required>
                  <option value="">Select Agent...</option>
                  {agents.map((a) => <option key={a.id} value={a.id}>{a.hostname}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Arguments</label>
                <input type="text" placeholder="BOF arguments" value={runArgs} onChange={(e) => setRunArgs(e.target.value)}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm font-mono" />
              </div>
              <button type="submit" className="w-full h-10 bg-emerald-600 hover:bg-emerald-700 rounded-2xl text-sm text-white font-medium transition-colors">Execute</button>
            </form>
          </div>
        </div>
      )}

      {showInfo && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowInfo(null)}>
          <div className="ui-card shadow-xl w-full max-w-md p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">BOF Info</div>
              <button onClick={() => setShowInfo(null)} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
                <i className="fa-solid fa-xmark"></i>
              </button>
            </div>
            <div className="space-y-3">
              <div className="flex justify-between py-2 border-b border-slate-100 dark:border-slate-700">
                <span className="text-sm text-slate-500">Name</span>
                <span className="text-sm font-medium text-slate-900 dark:text-slate-100 font-mono">{showInfo.Name || showInfo.name}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-slate-100 dark:border-slate-700">
                <span className="text-sm text-slate-500">Size</span>
                <span className="text-sm text-slate-900 dark:text-slate-100">{formatBytes(showInfo.Size ?? showInfo.size)}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-slate-100 dark:border-slate-700">
                <span className="text-sm text-slate-500">Architecture</span>
                <span className="text-sm text-slate-900 dark:text-slate-100">{showInfo.Architecture || showInfo.architecture || "x64"}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-slate-100 dark:border-slate-700">
                <span className="text-sm text-slate-500">Description</span>
                <span className="text-sm text-slate-900 dark:text-slate-100 max-w-[60%] text-right">{showInfo.Description || showInfo.description || "-"}</span>
              </div>
              <div className="flex justify-between py-2">
                <span className="text-sm text-slate-500">Uploaded</span>
                <span className="text-sm text-slate-900 dark:text-slate-100">{showInfo.CreatedAt || showInfo.created_at || "-"}</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {editTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setEditTarget(null)}>
          <div className="ui-card shadow-xl w-full max-w-md p-6" onClick={(e) => e.stopPropagation()}>
            <div className="text-lg font-semibold text-slate-900 dark:text-slate-100 mb-4">Edit BOF</div>
            <form onSubmit={handleEdit} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Name</label>
                <input type="text" value={editTarget.Name || editTarget.name || ""} onChange={(e) => setEditTarget({ ...editTarget, Name: e.target.value })}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm font-mono" />
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-600 dark:text-slate-300 mb-1.5">Description</label>
                <input type="text" value={editTarget.Description || editTarget.description || ""} onChange={(e) => setEditTarget({ ...editTarget, Description: e.target.value })}
                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-2xl px-4 h-10 text-sm" />
              </div>
              <button type="submit" className="w-full h-10 bg-indigo-600 hover:bg-indigo-700 rounded-2xl text-sm text-white font-medium transition-colors">Save</button>
            </form>
          </div>
        </div>
      )}

      {toastMsg && (
        <div className="fixed bottom-6 right-6 z-50 bg-slate-900 dark:bg-slate-700 text-white text-sm px-5 py-3 rounded-2xl shadow-xl animate-fade-in max-w-xs">
          {toastMsg}
        </div>
      )}
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Delete" cancelText="Cancel" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
