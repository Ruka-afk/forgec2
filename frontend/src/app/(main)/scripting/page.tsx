"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { EmptyState, PageHeader, PageSpinner, Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { BookOpen, Code, Eraser, History, Layers, Play, Save, Terminal, Trash2, X } from "lucide-react";

import type { Agent } from "@/types/agent";

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
  const { t } = useI18n();
  const [loading, setLoading] = useState(true);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [savedScripts, setSavedScripts] = useState<SavedScript[]>([]);
  const [runHistory, setRunHistory] = useState<RunHistory[]>([]);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [scriptName, setScriptName] = useState("");
  const [scriptCode, setScriptCode] = useState("-- Lua Script Example\nforgec2.log('Starting batch operation')\n\nlocal agents = forgec2.get_agents()\nforgec2.log('Agents: ' .. tostring(#agents))\n\nforgec2.set_output('Script execution complete')");
  const [scriptOutput, setScriptOutput] = useState("Waiting for execution...");
  const [running, setRunning] = useState(false);
  const [showTemplates, setShowTemplates] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const agentData = await api.get("/agents");
      setAgents((agentData.agents || []) as Agent[]);
    } catch { setAgents([]); }
    try {
      const scriptData = await api.get("/api/scripts");
      setSavedScripts((scriptData.scripts || []) as SavedScript[]);
    } catch { setSavedScripts([]); }
    try {
      const historyData = await api.get("/api/scripts/history");
      setRunHistory((historyData.history || []) as RunHistory[]);
    } catch { setRunHistory([]); }
    setLoading(false);
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

  const handleSaveScript = async () => {
    if (!scriptName.trim() || !scriptCode.trim()) return;
    try {
      await api.postJson("/api/scripts", { name: scriptName, code: scriptCode });
      loadData();
    } catch { toast.error("Failed to save script"); }
  };

  const handleDeleteScript = async (scriptId: string) => {
    try {
      await api.del(`/api/scripts/${scriptId}`);
      loadData();
    } catch { toast.error("Failed to delete script"); }
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
      const data = await api.postJson("/api/scripts/execute", { agent_id: selectedAgent, code: scriptCode, name: scriptName });
      setScriptOutput((data.output || data.result || data.error || "Script executed with no output") as string);
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
    return <PageSpinner />;
  }

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("scripting.title")} subtitle={t("scripting.subtitle")}>
        <Button onClick={() => setShowTemplates(!showTemplates)}
          className="bg-primary hover:bg-primary/90 text-primary-foreground">
          <BookOpen className="w-4 h-4" />
          <span className="hidden sm:inline">{t("scripting.templates")}</span>
        </Button>
      </PageHeader>

      {showTemplates && (
        <Card className="mb-6 p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Layers className="w-4 h-4" />
              <span className="text-sm font-semibold text-foreground">{t("scripting.script_library")}</span>
            </div>
            <Button variant="ghost" size="sm" onClick={() => setShowTemplates(false)} className="text-muted-foreground hover:text-foreground">
              <X className="w-4 h-4" />
            </Button>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {defaultTemplates.map((t) => (
              <Button key={t.name} variant="outline" onClick={() => handleApplyTemplate(t)}
                className="text-left p-4 h-auto bg-muted hover:border-purple-400 dark:hover:border-purple-500 transition-colors">
                <div className="text-sm font-medium text-foreground">{t.name}</div>
                <div className="text-xs text-muted-foreground mt-1">{t.description}</div>
              </Button>
            ))}
          </div>
        </Card>
      )}

      <div className="space-y-4">
        <Tabs defaultValue="editor">
        <TabsList className="mb-4">
          <TabsTrigger value="editor" className="gap-1.5">
            <Code className="w-4 h-4" />{t("scripting.editor")}
          </TabsTrigger>
          <TabsTrigger value="history" className="gap-1.5">
            <History className="w-4 h-4" />{t("scripting.run_history", { count: String(runHistory.length) })}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="editor">
          <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
            <div className="lg:col-span-1 space-y-4">
              <Card className="overflow-hidden">
                <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                  <div className="text-sm font-semibold text-foreground">{t("scripting.target_agent")}</div>
                </div>
                <div className="p-4">
                  <Label htmlFor="agent-select" className="sr-only">{t("scripting.select_agent")}</Label>
                  <Select value={selectedAgent} onValueChange={v => setSelectedAgent(v ?? "")}>
                    <SelectTrigger className="w-full"><SelectValue placeholder={t("scripting.select_agent_placeholder")} /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="">{t("scripting.select_agent_placeholder")}</SelectItem>
                      {agents.map(a => {
                        const aid = a.id || "";
                        const host = a.hostname || "";
                        const ip = a.ip || "";
                        return <SelectItem key={aid} value={aid}>{host} ({ip})</SelectItem>;
                      })}
                    </SelectContent>
                  </Select>
                </div>
              </Card>

              <Card className="overflow-hidden">
                <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                  <span className="text-sm font-semibold text-foreground">{t("scripting.saved_scripts")}</span>
                </div>
                <div className="max-h-60 overflow-y-auto">
                  {savedScripts.length === 0 ? (
                    <div className="p-4 text-center text-muted-foreground text-sm">{t("scripting.no_scripts")}</div>
                  ) : (
                    savedScripts.map((s) => (
                      <div key={s.id} className="flex items-center justify-between px-4 py-2.5 border-b border-border hover:bg-muted transition-colors">
                        <Button variant="ghost" size="sm" onClick={() => handleLoadScript(s)} className="text-sm text-muted-foreground hover:text-indigo-600 dark:hover:text-indigo-400 truncate text-left flex-1 justify-start">
                          {s.name || t("scripting.untitled")}
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => s.id && handleDeleteScript(s.id)} className="text-muted-foreground hover:text-destructive ml-2">
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    ))
                  )}
                </div>
              </Card>

              <Card className="overflow-hidden">
                <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border">
                  <span className="text-sm font-semibold text-foreground">{t("scripting.api_reference")}</span>
                </div>
                <div className="p-3 text-xs font-mono text-blue-600 dark:text-blue-400 space-y-1">
                  {apiRefs.map((ref) => (
                    <div key={ref} className="py-0.5">{ref}</div>
                  ))}
                </div>
              </Card>
            </div>

            <div className="lg:col-span-3">
              <Card className="overflow-hidden">
                <div className="px-4 py-3 sm:px-5 sm:py-3.5 border-b border-border flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Input id="script-name" placeholder={t("scripting.script_name_ph")} value={scriptName} onChange={e => setScriptName(e.target.value)}
                      className="bg-transparent border-none text-sm font-semibold focus:outline-none w-48 h-auto p-0" />
                  </div>
                  <div className="flex items-center gap-2">
                    <Button variant="outline" size="sm" onClick={handleSaveScript} disabled={!scriptName.trim()}>
                      <Save className="w-4 h-4" />{t("scripting.save")}
                    </Button>
                    <Button size="sm" onClick={handleRunScript} disabled={running || !selectedAgent || !scriptCode.trim()}
                      className="bg-emerald-600 hover:bg-emerald-700 text-white">
                      {running ? <Spinner size="xs" className="mr-1" /> : <Play className="w-4 h-4" />}
                      {running ? t("scripting.running") : t("scripting.run")}
                    </Button>
                  </div>
                </div>
                <div className="flex">
                  <div className="bg-muted text-muted-foreground text-xs font-mono py-4 px-2 text-right select-none min-w-[3rem] overflow-hidden">
                    {Array.from({ length: lineNumbers }, (_, i) => (
                      <div key={i + 1} className="leading-5 h-5">{i + 1}</div>
                    ))}
                  </div>
                  <Textarea
                    value={scriptCode}
                    onChange={e => setScriptCode(e.target.value)}
                    className="flex-1 h-72 p-4 font-mono text-sm bg-background text-emerald-300 resize-none focus:outline-none leading-5 border-none rounded-none"
                    placeholder={t("scripting.code_ph")}
                    spellCheck={false}
                  />
                </div>
                <div className="border-t border-border">
                  <div className="p-3 px-4 text-xs font-semibold text-muted-foreground flex items-center justify-between">
                    <span>
                      <Terminal className="w-4 h-4" />{t("scripting.output")}
                    </span>
                    <Button variant="ghost" size="sm" onClick={() => setScriptOutput("Waiting for execution...")} className="text-xs text-muted-foreground hover:text-foreground h-auto p-0">
                      <Eraser className="w-4 h-4" />{t("scripting.clear")}
                    </Button>
                  </div>
                  <pre className="p-4 text-xs font-mono text-emerald-300 bg-background m-2 rounded-lg max-h-48 overflow-y-auto whitespace-pre-wrap border border-border">
                    {scriptOutput}
                  </pre>
                </div>
              </Card>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="history">
          <Card className="overflow-hidden">
            {runHistory.length === 0 ? (
              <div className="text-center py-16 sm:py-20">
                <EmptyState icon={History} title={t("scripting.no_history")} />
              </div>
            ) : (
              <div className="divide-y divide-border">
                {runHistory.map((run, i) => (
                  <div key={run.id || i} className="p-4">
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-3">
                        <span className="text-sm font-medium text-foreground">{run.script_name || "Unknown"}</span>
                        <span className="text-xs text-muted-foreground">{run.agent_hostname || "Unknown"}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className={`text-[10px] px-2 py-0.5 rounded-full ${
                          (run.status || "") === "success"
                            ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                            : "bg-destructive/10 text-destructive"
                        }`}>{run.status || "completed"}</span>
                        <span className="text-xs text-muted-foreground">{run.created_at || ""}</span>
                      </div>
                    </div>
                    {run.output && (
                      <pre className="text-xs font-mono text-muted-foreground bg-background rounded-lg p-3 mt-2 max-h-32 overflow-y-auto whitespace-pre-wrap">{run.output}</pre>
                    )}
                  </div>
                ))}
              </div>
            )}
          </Card>
        </TabsContent>
      </Tabs>
      </div>
    </div>
  );
}

