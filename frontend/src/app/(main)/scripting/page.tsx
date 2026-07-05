"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";

interface Agent {
  id?: string;
  ID?: string;
  hostname?: string;
  Hostname?: string;
  ip?: string;
  IP?: string;
}

interface SavedScript {
  id?: string;
  name?: string;
  code?: string;
  created_at?: string;
}

interface RunHistory {
  id?: string;
  script_name?: string;
  agent_hostname?: string;
  status?: string;
  output?: string;
  created_at?: string;
}

interface ScriptTemplate {
  name: string;
  description: string;
  code: string;
}

const defaultTemplates: ScriptTemplate[] = [
  { name: "Agent Enumeration", description: "List all active agents", code: "forgec2.log('Enumerating agents...')\nlocal agents = forgec2.get_agents()\nfor i, a in ipairs(agents) do\n  forgec2.log(a.hostname .. ' @ ' .. a.ip)\nend\nforgec2.set_output('Found ' .. #agents .. ' agents')" },
  { name: "Credential Harvest", description: "Extract credentials from vault", code: "forgec2.log('Harvesting credentials...')\nlocal creds = forgec2.get_credentials()\nfor i, c in ipairs(creds) do\n  forgec2.log(c.username .. '@' .. c.target)\nend\nforgec2.set_output('Found ' .. #creds .. ' credentials')" },
  { name: "Bulk Task Creation", description: "Create tasks for all agents", code: "forgec2.log('Creating bulk tasks...')\nlocal agents = forgec2.get_agents()\nlocal cmd = 'whoami /all'\nfor i, a in ipairs(agents) do\n  forgec2.create_task(a.id, 'exec', cmd)\nend\nforgec2.set_output('Tasks created for ' .. #agents .. ' agents')" },
  { name: "Sleep & Check", description: "Wait for agents to callback", code: "forgec2.log('Sleeping 30s for callbacks...')\nforgec2.sleep(30000)\nlocal agents = forgec2.get_agents()\nforgec2.log('Active after sleep: ' .. #agents)\nforgec2.set_output('Callback check complete')" },
  { name: "Network Discovery", description: "Run network scan on all agents", code: "forgec2.log('Starting network discovery...')\nlocal agents = forgec2.get_agents()\nfor i, a in ipairs(agents) do\n  forgec2.create_task(a.id, 'powershell', 'Test-NetComputer -Port 445')\nend\nforgec2.set_output('Discovery tasks queued')" },
];

export default function ScriptingPage() {
  const [loading, setLoading] = useState(true);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [savedScripts, setSavedScripts] = useState<SavedScript[]>([]);
  const [runHistory, setRunHistory] = useState<RunHistory[]>([]);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [scriptName, setScriptName] = useState("");
  const [scriptCode, setScriptCode] = useState("-- Lua Script Example\nforgec2.log('Starting batch operation')\n\nlocal agents = forgec2.get_agents()\nforgec2.log('Agents: ' .. tostring(#agents))\n\nforgec2.set_output('Script execution complete')");
  const [scriptOutput, setScriptOutput] = useState("Waiting for execution...");
  const [running, setRunning] = useState(false);
  const [activeTab, setActiveTab] = useState<"editor" | "history">("editor");
  const [showTemplates, setShowTemplates] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const agentRes = await fetch(`${API_BASE}?p=/agents&format=json`);
      if (!agentRes.ok) throw new Error(`HTTP ${agentRes.status}`);
      const agentData = await agentRes.json();
      setAgents(agentData.Agents || agentData.agents || []);
    } catch { setAgents([]); }
    try {
      const scriptRes = await fetch(`${API_BASE}?p=/api/scripts&format=json`);
      if (!scriptRes.ok) throw new Error(`HTTP ${scriptRes.status}`);
      const scriptData = await scriptRes.json();
      setSavedScripts(scriptData.scripts || []);
    } catch { setSavedScripts([]); }
    try {
      const historyRes = await fetch(`${API_BASE}?p=/api/scripts/history&format=json`);
      if (!historyRes.ok) throw new Error(`HTTP ${historyRes.status}`);
      const historyData = await historyRes.json();
      setRunHistory(historyData.history || []);
    } catch { setRunHistory([]); }
    setLoading(false);
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadData()); }, [loadData]);

  const handleSaveScript = async () => {
    if (!scriptName.trim() || !scriptCode.trim()) return;
    try {
      await fetch(`${API_BASE}?p=/api/scripts&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: scriptName, code: scriptCode }),
      });
      loadData();
    } catch (e) { console.error("Scripting: save script failed", e); }
  };

  const handleDeleteScript = async (scriptId: string) => {
    try {
      await fetch(`${API_BASE}?p=/api/scripts/${scriptId}&format=json`, { method: "DELETE" });
      loadData();
    } catch (e) { console.error("Scripting: delete script failed", e); }
  };

  const handleLoadScript = (script: SavedScript) => {
    setScriptName(script.name || "");
    setScriptCode(script.code || "");
  };

  const handleRunScript = async () => {
    if (!selectedAgent || !scriptCode.trim()) return;
    setRunning(true);
    setScriptOutput("Executing script...");
    try {
      const res = await fetch(`${API_BASE}?p=/api/scripts/execute&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ agent_id: selectedAgent, code: scriptCode, name: scriptName }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setScriptOutput(data.output || data.result || data.error || "Script executed with no output");
      loadData();
    } catch {
      setScriptOutput("Error: Failed to execute script");
    }
    setRunning(false);
  };

  const handleApplyTemplate = (template: ScriptTemplate) => {
    setScriptName(template.name);
    setScriptCode(template.code);
    setShowTemplates(false);
  };

  const lineNumbers = scriptCode.split("\n").length;

  const apiRefs = [
    "forgec2.log(msg)",
    "forgec2.get_agents()",
    "forgec2.create_task(agent, type, cmd)",
    "forgec2.get_tasks()",
    "forgec2.get_credentials()",
    "forgec2.sleep(ms)",
    "forgec2.set_output(str)",
  ];

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <i className="fa-solid fa-circle-notch fa-spin text-3xl text-indigo-500"></i>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-6 gap-3">
        <div>
          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">Script Console</h1>
          <p className="text-slate-500 dark:text-slate-400 text-xs sm:text-sm mt-1">Lua script automation &middot; Batch operations &middot; Custom workflows</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => setShowTemplates(!showTemplates)}
            className="px-4 h-10 bg-purple-600 hover:bg-purple-700 text-white rounded-xl text-sm font-medium transition-colors flex items-center gap-2">
            <i className="fa-solid fa-book"></i>
            Templates
          </button>
        </div>
      </div>

      {showTemplates && (
        <div className="mb-6 ui-card p-5 shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <i className="fa-solid fa-layer-group text-purple-600 dark:text-purple-400"></i>
              <span className="text-sm font-semibold text-slate-900 dark:text-slate-100">Script Library</span>
            </div>
            <button onClick={() => setShowTemplates(false)} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
              <i className="fa-solid fa-xmark"></i>
            </button>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {defaultTemplates.map((t) => (
              <button key={t.name} onClick={() => handleApplyTemplate(t)}
                className="text-left p-4 bg-slate-50 dark:bg-slate-700/50 rounded-xl border border-[var(--border)] hover:border-purple-400 dark:hover:border-purple-500 transition-colors">
                <div className="text-sm font-medium text-slate-900 dark:text-slate-100">{t.name}</div>
                <div className="text-xs text-slate-500 dark:text-slate-400 mt-1">{t.description}</div>
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="space-y-4">
        <div className="flex gap-1 ui-card p-1.5">
          <button onClick={() => setActiveTab("editor")}
            className={`px-4 py-2 text-sm font-medium rounded-xl transition-colors ${activeTab === "editor" ? "bg-indigo-50 text-indigo-600 dark:bg-indigo-900/20 dark:text-indigo-400" : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"}`}>
            <i className="fa-solid fa-code mr-1.5"></i>Editor
          </button>
          <button onClick={() => setActiveTab("history")}
            className={`px-4 py-2 text-sm font-medium rounded-xl transition-colors ${activeTab === "history" ? "bg-indigo-50 text-indigo-600 dark:bg-indigo-900/20 dark:text-indigo-400" : "text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"}`}>
            <i className="fa-solid fa-clock-rotate-left mr-1.5"></i>Run History ({runHistory.length})
          </button>
        </div>

        {activeTab === "editor" && (
          <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
            <div className="lg:col-span-1 space-y-4">
              <div className="ui-card overflow-hidden">
                <div className="p-4 border-b border-[var(--border)]">
                  <div className="text-sm font-semibold text-slate-900 dark:text-slate-100">Target Agent</div>
                </div>
                <div className="p-4">
                  <select value={selectedAgent} onChange={e => setSelectedAgent(e.target.value)}
                    className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2.5 focus:outline-none focus:border-indigo-500 dark:text-slate-100">
                    <option value="">-- Select Agent --</option>
                    {agents.map(a => {
                      const aid = a.ID || a.id || "";
                      const host = a.Hostname || a.hostname || "";
                      const ip = a.IP || a.ip || "";
                      return <option key={aid} value={aid}>{host} ({ip})</option>;
                    })}
                  </select>
                </div>
              </div>

              <div className="ui-card overflow-hidden">
                <div className="p-4 border-b border-[var(--border)]">
                  <span className="text-sm font-semibold text-slate-900 dark:text-slate-100">Saved Scripts</span>
                </div>
                <div className="max-h-60 overflow-y-auto">
                  {savedScripts.length === 0 ? (
                    <div className="p-4 text-center text-slate-400 text-sm">No scripts saved</div>
                  ) : (
                    savedScripts.map((s) => (
                      <div key={s.id} className="flex items-center justify-between px-4 py-2.5 border-b border-slate-100 dark:border-slate-700 hover:bg-slate-50 dark:hover:bg-slate-700/50">
                        <button onClick={() => handleLoadScript(s)} className="text-sm text-[var(--text-secondary)] hover:text-indigo-600 dark:hover:text-indigo-400 truncate text-left flex-1">
                          {s.name || "Untitled"}
                        </button>
                        <button onClick={() => s.id && handleDeleteScript(s.id)} className="text-slate-400 hover:text-red-500 ml-2">
                          <i className="fa-solid fa-trash text-xs"></i>
                        </button>
                      </div>
                    ))
                  )}
                </div>
              </div>

              <div className="ui-card overflow-hidden">
                <div className="p-4 border-b border-[var(--border)]">
                  <span className="text-sm font-semibold text-slate-900 dark:text-slate-100">API Reference</span>
                </div>
                <div className="p-3 text-xs font-mono text-blue-600 dark:text-blue-400 space-y-1">
                  {apiRefs.map((ref) => (
                    <div key={ref} className="py-0.5">{ref}</div>
                  ))}
                </div>
              </div>
            </div>

            <div className="lg:col-span-3">
              <div className="ui-card overflow-hidden">
                <div className="p-4 border-b border-[var(--border)] flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <input type="text" placeholder="Script name..." value={scriptName} onChange={e => setScriptName(e.target.value)}
                      className="bg-transparent border-none text-sm font-semibold text-slate-900 dark:text-slate-100 focus:outline-none w-48" />
                  </div>
                  <div className="flex items-center gap-2">
                    <button onClick={handleSaveScript} disabled={!scriptName.trim()}
                      className="px-3 py-1.5 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 disabled:opacity-50 disabled:cursor-not-allowed text-[var(--text-secondary)] rounded-lg text-xs font-medium transition-colors">
                      <i className="fa-solid fa-floppy-disk mr-1"></i>Save
                    </button>
                    <button onClick={handleRunScript} disabled={running || !selectedAgent || !scriptCode.trim()}
                      className="px-3 py-1.5 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg text-xs font-medium transition-colors">
                      <i className={`fa-solid ${running ? "fa-circle-notch fa-spin" : "fa-play"} mr-1`}></i>
                      {running ? "Running..." : "Run"}
                    </button>
                  </div>
                </div>
                <div className="flex">
                  <div className="bg-slate-800 dark:bg-slate-900 text-slate-500 text-xs font-mono py-4 px-2 text-right select-none min-w-[3rem] overflow-hidden">
                    {Array.from({ length: lineNumbers }, (_, i) => (
                      <div key={i + 1} className="leading-5 h-5">{i + 1}</div>
                    ))}
                  </div>
                  <textarea
                    value={scriptCode}
                    onChange={e => setScriptCode(e.target.value)}
                    className="flex-1 h-72 p-4 font-mono text-sm bg-slate-900 text-emerald-300 resize-none focus:outline-none leading-5"
                    placeholder="Enter Lua script..."
                    spellCheck={false}
                  />
                </div>
                <div className="border-t border-[var(--border)]">
                  <div className="p-3 px-4 text-xs font-semibold text-slate-500 flex items-center justify-between">
                    <span>
                      <i className="fa-solid fa-terminal mr-1"></i>Output
                    </span>
                    <button onClick={() => setScriptOutput("Waiting for execution...")} className="text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-200">
                      <i className="fa-solid fa-eraser mr-1"></i>Clear
                    </button>
                  </div>
                  <pre className="p-4 text-xs font-mono text-emerald-300 bg-slate-900 m-2 rounded-lg max-h-48 overflow-y-auto whitespace-pre-wrap border border-slate-700">
                    {scriptOutput}
                  </pre>
                </div>
              </div>
            </div>
          </div>
        )}

        {activeTab === "history" && (
          <div className="ui-card overflow-hidden">
            {runHistory.length === 0 ? (
              <div className="text-center py-12 text-slate-400 dark:text-slate-500">
                <i className="fa-solid fa-clock-rotate-left text-3xl mb-3 text-slate-300 dark:text-slate-600"></i>
                <p>No run history yet</p>
              </div>
            ) : (
              <div className="divide-y divide-slate-100 dark:divide-slate-700">
                {runHistory.map((run, i) => (
                  <div key={run.id || i} className="p-4">
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-3">
                        <span className="text-sm font-medium text-slate-900 dark:text-slate-100">{run.script_name || "Unknown"}</span>
                        <span className="text-xs text-slate-500">{run.agent_hostname || "Unknown"}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className={`text-[10px] px-2 py-0.5 rounded-full ${
                          (run.status || "") === "success"
                            ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                            : "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                        }`}>{run.status || "completed"}</span>
                        <span className="text-xs text-slate-500">{run.created_at || ""}</span>
                      </div>
                    </div>
                    {run.output && (
                      <pre className="text-xs font-mono text-slate-300 bg-slate-900 rounded-lg p-3 mt-2 max-h-32 overflow-y-auto whitespace-pre-wrap">{run.output}</pre>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
