"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { useI18n } from "@/lib/i18n";
import { API_BASE } from "@/lib/constants";
import { downloadText } from "@/lib/download";
import { api, getCsrfToken } from "@/lib/api";
import { renderMarkdown } from "@/lib/markdown";
import { sanitizeHtml } from "@/lib/sanitize";
import { CopyButton, Spinner, MdContent, PageHeader } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { Bot, Brain, Wand2, Check, Copy, Download, Menu, RotateCw, Send, Settings, Square, Terminal, User, Wrench, X } from "lucide-react";
import AISessionSidebar from "./_components/AISessionSidebar";

function SanitizedMarkdown({ content }: { content: string }) {
  const [safe, setSafe] = useState<string>("");
  useEffect(() => {
    let cancelled = false;
    sanitizeHtml(renderMarkdown(content)).then((h) => { if (!cancelled) setSafe(h); });
    return () => { cancelled = true; };
  }, [content]);
  return <MdContent dangerouslySetInnerHTML={{ __html: safe }} />;
}

interface AIMessage {
  role: "user" | "assistant" | "tool";
  content: string;
  tool_name?: string;
  thinking?: boolean;
}

interface AIConfig {
  enabled: boolean;
  provider: string;
  api_key: string;
  model: string;
  endpoint: string;
  system_prompt: string;
  allow_execute?: boolean;
}

interface AISession {
  id: number;
  title: string;
  updated_at: string;
}

export default function AIPage() {
  const { t } = useI18n();
  const [messages, setMessages] = useState<AIMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [provider, setProvider] = useState("deepseek");
  const [model, setModel] = useState("deepseek-chat");
  const [apiKey, setApiKey] = useState("");
  const [endpoint, setEndpoint] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");
  const [allowExecute, setAllowExecute] = useState(false);
  const [configSaving, setConfigSaving] = useState(false);
  const [sessions, setSessions] = useState<AISession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<number | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [renameTarget, setRenameTarget] = useState<{ id: number; current: string } | null>(null);
  const [renameValue, setRenameValue] = useState("");
  const abortRef = useRef<AbortController | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const adjustTextarea = () => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 128) + "px";
  };

  // messagesRef mirrors `messages` synchronously so streaming/autosave can read
  // the latest value without stale closures.
  const messagesRef = useRef<AIMessage[]>([]);
  const setMessagesBoth = useCallback(
    (updater: AIMessage[] | ((prev: AIMessage[]) => AIMessage[])) => {
      setMessages((prev) => {
        const next =
          typeof updater === "function"
            ? (updater as (p: AIMessage[]) => AIMessage[])(prev)
            : updater;
        const capped = next.length > 500 ? next.slice(-500) : next;
        messagesRef.current = capped;
        return capped;
      });
    },
    []
  );

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => { scrollToBottom(); }, [messages]);
  useEffect(() => { adjustTextarea(); }, [input]);

  const loadConfig = useCallback(async () => {
    try {
      const data = await api.get("/ai");
      if (data.AIConfig) {
        const cfg = data.AIConfig as AIConfig;
        if (cfg.provider) setProvider(cfg.provider);
        if (cfg.model) setModel(cfg.model);
        if (cfg.endpoint) setEndpoint(cfg.endpoint);
        if (cfg.system_prompt) setSystemPrompt(cfg.system_prompt);
        setAllowExecute(Boolean(cfg.allow_execute));
      }
    } catch { toast.error(t("ai.toast.load_config_failed")); }
  }, [t]);

  const loadSessions = useCallback(async () => {
    try {
      const data = await api.get("/ai/sessions");
      const list = Array.isArray(data) ? data : (data?.data ?? []);
      setSessions(list as AISession[]);
    } catch { toast.error(t("ai.toast.load_sessions_failed")); }
  }, [t]);

  useEffect(() => { loadConfig(); }, [loadConfig]);
  useEffect(() => { loadSessions(); }, [loadSessions]);

  const handleSaveConfig = async () => {
    try {
      setConfigSaving(true);
      const data = await api.postJson("/ai/config", { enabled: true, provider, model, api_key: apiKey, endpoint, system_prompt: systemPrompt, allow_execute: allowExecute });
      if (data.success) {
        setShowSettings(false);
      }
    } catch { toast.error(t("ai.toast.save_config_failed")); } finally {
      setConfigSaving(false);
    }
  };

  const selectSession = async (id: number) => {
    try {
      const data = await api.get(`/ai/sessions/${id}/messages`);
      const list = Array.isArray(data) ? data : (data?.data ?? []);
      const mapped = (list as { role: string; content: string; tool_name?: string }[]).map(
        (m) => ({ role: m.role as AIMessage["role"], content: m.content, tool_name: m.tool_name, thinking: false })
      );
      setMessagesBoth(mapped);
      setActiveSessionId(id);
      setSidebarOpen(false);
    } catch { toast.error(t("ai.toast.load_session_messages_failed")); }
  };

  const deleteSession = async (id: number) => {
    try {
      await api.del(`/ai/sessions/${id}`);
      setSessions((prev) => prev.filter((s) => s.id !== id));
      if (activeSessionId === id) {
        setActiveSessionId(null);
        setMessagesBoth([]);
      }
    } catch { toast.error(t("ai.toast.delete_session_failed")); }
  };

  const handleNewChat = () => {
    setMessagesBoth([]);
    setActiveSessionId(null);
    setSidebarOpen(false);
  };

  const renameSession = (id: number, current: string) => {
    setRenameTarget({ id, current });
    setRenameValue(current);
  };

  const confirmRename = async () => {
    if (!renameTarget) return;
    const title = renameValue.trim();
    if (title === "" || title === renameTarget.current) { setRenameTarget(null); return; }
    try {
      await api.putJson(`/ai/sessions/${renameTarget.id}`, { title });
      setSessions((prev) => prev.map((s) => (s.id === renameTarget.id ? { ...s, title } : s)));
    } catch { toast.error(t("ai.toast.rename_session_failed")); }
    setRenameTarget(null);
  };

  // handleRegenerate trims the trailing assistant/tool reply and re-asks the last
  // user message, reusing the existing streaming path via handleSend.
  const handleRegenerate = () => {
    if (loading) return;
    const cur = messagesRef.current;
    let lastUser = -1;
    for (let i = cur.length - 1; i >= 0; i--) {
      if (cur[i].role === "user") { lastUser = i; break; }
    }
    if (lastUser < 0) return;
    const lastUserMsg = cur[lastUser];
    setMessagesBoth(cur.slice(0, lastUser + 1));
    setInput(lastUserMsg.content);
    setTimeout(() => handleSend(), 0);
  };

  // saveTurn persists every message added since `fromIndex` into the session.
  const saveTurn = async (sessionId: number, fromIndex: number) => {
    const msgs = messagesRef.current.slice(fromIndex);
    for (const m of msgs) {
      if (m.thinking) continue;
      try {
        await api.postJson(`/ai/sessions/${sessionId}/messages`, {
          role: m.role,
          content: m.content,
          tool_name: m.tool_name || "",
        });
      } catch { toast.error(t("ai.toast.save_message_failed")); }
    }
  };

  const handleSend = async () => {
    if (!input.trim() || loading) return;
    const text = input.trim();
    const userMsg: AIMessage = { role: "user", content: text };
    const userIndex = messagesRef.current.length;
    setMessagesBoth((prev) => [...prev, userMsg]);
    setInput("");
    setLoading(true);

    let sessionId = activeSessionId;
    if (sessionId == null) {
      try {
        const created = await api.postJson<{ id: number; title: string }>("/ai/sessions", {
          title: text.slice(0, 40),
        });
        sessionId = created?.id ?? null;
        if (sessionId != null) {
          setActiveSessionId(sessionId);
          loadSessions();
        }
      } catch {
        sessionId = null;
        toast.error(t("ai.toast.create_session_failed"));
      }
    }

    const controller = new AbortController();
    abortRef.current = controller;

    const thinkingMsg: AIMessage = { role: "assistant", content: "", thinking: true };
    setMessagesBoth((prev) => [...prev, thinkingMsg]);

    try {
      const conversationHistory = [...messagesRef.current, userMsg];
      const response = await fetch(`${API_BASE}/ai/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": getCsrfToken() },
        body: JSON.stringify({ messages: conversationHistory }),
        credentials: "include",
        signal: controller.signal,
      });

      if (!response.ok || !response.body) {
        setMessagesBoth((prev) => {
          const updated = [...prev];
          const idx = updated.findIndex((m) => m.thinking);
          if (idx !== -1) {
            updated[idx] = { role: "assistant", content: t("ai.error_connection") };
          } else {
            updated.push({ role: "assistant", content: t("ai.error_connection") });
          }
          return updated;
        });
        return;
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      let currentContent = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });

        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        let currentEvent = "";
        for (const line of lines) {
          if (line.startsWith("event: ")) {
            currentEvent = line.substring(7).trim();
          } else if (line.startsWith("data: ")) {
            const data = line.substring(6);

            switch (currentEvent) {
              case "thinking": {
                setMessagesBoth((prev) => {
                  const updated = [...prev];
                  const idx = updated.findIndex((m) => m.thinking);
                  if (idx !== -1) {
                    updated[idx] = { role: "assistant", content: t("ai.thinking"), thinking: true };
                  }
                  return updated;
                });
                break;
              }
              case "clear": {
                setMessagesBoth((prev) => {
                  const updated = [...prev];
                  const idx = updated.findIndex((m) => m.thinking);
                  if (idx !== -1) {
                    updated[idx] = { role: "assistant", content: "", thinking: false };
                  }
                  return updated;
                });
                currentContent = "";
                break;
              }
              case "text": {
                currentContent = data;
                setMessagesBoth((prev) => {
                  const updated = [...prev];
                  const idx = updated.findIndex((m) => m.role === "assistant" && !m.thinking);
                  if (idx !== -1) {
                    updated[idx] = { role: "assistant", content: currentContent };
                  } else if (updated.some((m) => m.thinking)) {
                    const thinkIdx = updated.findIndex((m) => m.thinking);
                    updated[thinkIdx] = { role: "assistant", content: currentContent };
                  } else {
                    updated.push({ role: "assistant", content: currentContent });
                  }
                  return updated;
                });
                break;
              }
              case "reasoning": {
                setMessagesBoth((prev) => {
                  const updated = [...prev];
                  const idx = updated.findIndex((m) => m.thinking);
                  if (idx !== -1) {
                    updated[idx] = { role: "assistant", content: data, thinking: true };
                  }
                  return updated;
                });
                break;
              }
              case "tool_start": {
                const toolLabel = `${t("ai.calling_tool")} ${data}...`;
                setMessagesBoth((prev) => [
                  ...prev,
                  { role: "tool", content: toolLabel, tool_name: data },
                ]);
                break;
              }
              case "tool": {
                try {
                  const parsed = JSON.parse(data);
                  const resultLabel = `${t("ai.tool_result")}:\n${parsed.result}`;
                  setMessagesBoth((prev) => {
                    const updated = [...prev];
                    const idx = updated.findIndex(
                      (m) => m.role === "tool" && m.tool_name === parsed.name
                    );
                    if (idx !== -1) {
                      updated[idx] = { role: "tool", content: resultLabel, tool_name: parsed.name };
                    } else {
                      updated.push({ role: "tool", content: resultLabel, tool_name: parsed.name });
                    }
                    return updated;
                  });
                } catch {
                  setMessagesBoth((prev) => [
                    ...prev,
                    { role: "tool", content: data, tool_name: t("ai.tool") },
                  ]);
                }
                break;
              }
              case "error": {
                setMessagesBoth((prev) => [
                  ...prev,
                  { role: "assistant", content: t("ai.error_prefix") + data },
                ]);
                break;
              }
              default:
                break;
            }
          }
        }
      }
    } catch (err: unknown) {
      if (err instanceof Error && err.name === "AbortError") {
        return;
      }
      setMessagesBoth((prev) => {
        const updated = [...prev];
        const idx = updated.findIndex((m) => m.thinking);
        if (idx !== -1) {
          updated[idx] = { role: "assistant", content: t("ai.error_unavailable") };
        } else {
          updated.push({ role: "assistant", content: t("ai.error_unavailable") });
        }
        return updated;
      });
    } finally {
      setLoading(false);
      abortRef.current = null;
      if (sessionId != null) {
        await saveTurn(sessionId, userIndex);
        loadSessions();
      }
    }
  };

  const handleStop = () => {
    if (abortRef.current) {
      abortRef.current.abort();
      abortRef.current = null;
    }
    setLoading(false);
  };

  const handleClear = () => {
    setMessagesBoth([]);
    setActiveSessionId(null);
  };

  const handleExport = () => {
    const text = messages
      .filter((m) => !m.thinking)
      .map((m) => {
        const roleLabel =
          m.role === "user" ? "[user]" : m.role === "assistant" ? "[assistant]" : `[tool:${m.tool_name}]`;
        return `${roleLabel} ${m.content}`;
      })
      .join("\n\n");
    downloadText(text, "ai-chat-export.txt");
  };

  const quickActions = [
    { label: t("ai.quick_list_implants"), query: t("ai.quick_list_implants") },
    { label: t("ai.quick_list_listeners"), query: t("ai.quick_list_listeners") },
    { label: t("ai.quick_credential_summary"), query: t("ai.quick_credential_summary") },
    { label: t("ai.quick_who_online"), query: t("ai.quick_who_online") },
  ];

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const lastAssistantIndex = (() => {
    let idx = -1;
    messages.forEach((m, i) => {
      if (m.role === "assistant" && !m.thinking) idx = i;
    });
    return idx;
  })();

  return (
    <div className="animate-fade-slide-up h-full min-h-0 flex w-full gap-3 pb-20 md:pb-0">
      {/* Desktop session sidebar */}
      <aside className="hidden md:flex w-56 shrink-0 flex-col border border-border rounded-2xl p-2">
        <AISessionSidebar
          sessions={sessions}
          activeSessionId={activeSessionId}
          onSelect={selectSession}
          onDelete={deleteSession}
          onRename={renameSession}
          onNewChat={handleNewChat}
        />
      </aside>

      {/* Mobile session sidebar */}
      <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
        <SheetContent side="left" className="w-64 p-0">
          <AISessionSidebar
            sessions={sessions}
            activeSessionId={activeSessionId}
            onSelect={(id) => { selectSession(id); setSidebarOpen(false); }}
            onDelete={deleteSession}
            onRename={renameSession}
            onNewChat={() => { handleNewChat(); setSidebarOpen(false); }}
          />
        </SheetContent>
      </Sheet>

      <div className="flex-1 min-w-0 min-h-0 flex flex-col max-w-(--content-width) mx-auto w-full">
        <div className="shrink-0 flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-3 px-1">
          <div className="min-w-0 flex items-center gap-2">
            <Button
              variant="outline"
              size="icon"
              onClick={() => setSidebarOpen(true)}
              className="md:hidden"
              aria-label="Toggle sessions"
            >
              <Menu className="w-4 h-4" />
            </Button>
            <PageHeader title={<><span className="w-9 h-9 rounded-xl bg-indigo-100 dark:bg-indigo-900/40 flex items-center justify-center shrink-0"><Bot className="w-4 h-4" /></span>{t("nav.ai")}</>} />
          </div>
          <div className="flex items-center gap-2 flex-wrap shrink-0">
            <Button variant="secondary" size="sm" onClick={() => setShowSettings(!showSettings)}>
              <Settings className="w-4 h-4" /> {t("common.edit")}
            </Button>
            <Button variant="destructive" size="sm" onClick={handleClear}>
              <Wand2 className="w-4 h-4" /> {t("ai.clear")}
            </Button>
            <Button variant="secondary" size="sm" onClick={handleExport}>
              <Download className="w-4 h-4" /> {t("ai.export")}
            </Button>
          </div>
        </div>

        <div className="shrink-0 mb-1 px-1">
          <span className="text-(--font-size-xs-sm) text-muted-foreground">
            {activeSessionId != null ? `${t("ai.sessions")} #${activeSessionId}` : t("ai.new_chat")}
          </span>
        </div>

        {showSettings && (
          <Card className="shrink-0 mb-3 p-4 sm:p-5">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-sm font-semibold text-foreground">{t("ai.config_title")}</h2>
              <Button variant="ghost" size="icon" onClick={() => setShowSettings(false)} aria-label="Close settings">
                <X className="w-4 h-4" />
              </Button>
            </div>
            <div className="space-y-4">
              <div className="flex items-center gap-3">
                <span className="flex items-center gap-2 cursor-pointer">
                  <span className="w-2 h-2 rounded-full bg-emerald-500"></span>
                  <span className="text-sm text-muted-foreground">{t("ai.enable_ai")}</span>
                </span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <span className="block text-xs text-muted-foreground mb-1">{t("ai.provider")}</span>
                  <Select value={provider} onValueChange={(v) => v && setProvider(v)}>
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="deepseek">DeepSeek</SelectItem>
                      <SelectItem value="openai">OpenAI</SelectItem>
                      <SelectItem value="claude">Claude</SelectItem>
                      <SelectItem value="qianwen">Qianwen</SelectItem>
                      <SelectItem value="longcat">LongCat</SelectItem>
                      <SelectItem value="custom">Custom</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <span className="block text-xs text-muted-foreground mb-1">{t("ai.model")}</span>
                  <Input type="text" value={model} onChange={(e) => setModel(e.target.value)} className="font-mono w-full" />
                </div>
                <div className="md:col-span-2">
                  <span className="block text-xs text-muted-foreground mb-1">{t("ai.endpoint") || "API Endpoint (optional)"}</span>
                  <Input type="text" value={endpoint} onChange={(e) => setEndpoint(e.target.value)} placeholder="https://api.openai.com/v1" className="font-mono w-full text-xs" />
                </div>
                <div className="md:col-span-2">
                  <span className="block text-xs text-muted-foreground mb-1">{t("ai.api_key") || "API Key"}</span>
                  <Input type="password" autoComplete="new-password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} placeholder="sk-..." className="font-mono w-full text-xs" />
                  <span className="block text-(--font-size-xs-sm) text-muted-foreground mt-0.5">{t("ai.api_key_hint") || "Leave blank to keep the existing key. The key is never returned to the client."}</span>
                </div>
              </div>
              <div>
                <span className="block text-xs text-muted-foreground mb-1">{t("ai.system_prompt") || "System Prompt (optional)"}</span>
                <Textarea value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} rows={3} className="w-full text-xs resize-y" placeholder="You are a helpful C2 operations assistant..." />
              </div>
              <Label className="flex items-start gap-3 cursor-pointer select-none">
                <Checkbox checked={allowExecute} onCheckedChange={(v) => setAllowExecute(v === true)} className="mt-1" />
                <span>
                  <span className="text-sm text-muted-foreground">{t("ai.allow_execute") || "Allow AI to execute commands on agents"}</span>
                  <span className="block text-(--font-size-xs-sm) text-amber-600 dark:text-amber-400 mt-0.5">{t("ai.allow_execute_warn") || "Warning: the assistant can run commands on your agents. Off by default."}</span>
                </span>
              </Label>
              <Button type="button" onClick={handleSaveConfig} disabled={configSaving} className="w-full h-10">
                {configSaving ? <><Spinner size="xs" className="mr-2" />{t("common.saving")}</> : t("common.save")}
              </Button>
            </div>
          </Card>
        )}

        <div className="flex-1 min-h-0 overflow-y-auto space-y-4 mb-3 pr-1 scroll-smooth rounded-xl border border-border bg-card/50 p-3 sm:p-4">
          {messages.length === 0 ? (
            <div className="flex gap-3">
              <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/40 rounded-xl flex items-center justify-center shrink-0 mt-1">
                <Bot className="w-4 h-4" />
              </div>
              <Card className="px-4 py-3 max-w-[90%] border border-border">
                <p className="text-sm text-muted-foreground font-medium">{t("ai.greeting_title")}</p>
                <p className="text-sm text-muted-foreground mt-1">{t("ai.greeting_desc")}</p>
                <div className="flex flex-wrap gap-2 mt-3">
                  {quickActions.map((q) => (
                    <Button key={q.label} variant="ghost" size="xs" onClick={() => { setInput(q.query); }} data-query={q.query}>
                      {q.label}
                    </Button>
                  ))}
                </div>
              </Card>
            </div>
          ) : (
            messages.map((msg, i) => {
              if (msg.thinking) {
                return (
                  <div key={i} className="flex gap-3">
                    <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/40 rounded-xl flex items-center justify-center shrink-0">
                      <Bot className="w-4 h-4" />
                    </div>
                    <Card className="rounded-tl-md px-4 py-3">
                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <Brain className="w-4 h-4" />
                        <span>{t("ai.thinking")}</span>
                        <span className="flex gap-1 ml-1">
                          <span className="w-1.5 h-1.5 bg-indigo-400 rounded-full animate-bounce"></span>
                          <span className="w-1.5 h-1.5 bg-indigo-400 rounded-full animate-bounce delay-150"></span>
                          <span className="w-1.5 h-1.5 bg-indigo-400 rounded-full animate-bounce delay-300"></span>
                        </span>
                      </div>
                    </Card>
                  </div>
                );
              }
              return (
                <div key={i} className={`flex gap-3 ${msg.role === "user" ? "flex-row-reverse" : ""}`}>
                  <div className={`w-8 h-8 rounded-xl flex items-center justify-center shrink-0 mt-1 ${msg.role === "user" ? "bg-secondary" : msg.role === "tool" ? "bg-amber-100 dark:bg-amber-900/40" : "bg-indigo-100 dark:bg-indigo-900/40"}`}>
                    {msg.role === "user" ? <User className="w-4 h-4 text-muted-foreground" /> : msg.role === "tool" ? <Wrench className="w-4 h-4 text-amber-500" /> : <Bot className="w-4 h-4 text-indigo-500 dark:text-indigo-400" />}
                  </div>
                  <div className={`max-w-[80%] ${msg.role === "user" ? "bg-indigo-600 text-white rounded-2xl rounded-tr-md" : "bg-card text-card-foreground ring-1 ring-foreground/10 rounded-tl-md"} px-4 py-3`}>
                    <div className="flex items-center justify-between gap-2 mb-1">
                      {msg.tool_name ? (
                        <div className="text-(--font-size-micro-sm) font-mono text-amber-600 dark:text-amber-400 flex items-center gap-1">
                          <Terminal className="w-4 h-4" />
                          {msg.tool_name}
                        </div>
                      ) : (
                        <span />
                      )}
                      <div className="flex items-center gap-2">
                        {msg.role === "assistant" && !msg.thinking && i === lastAssistantIndex && (
                          <Button
                            variant="ghost"
                            size="xs"
                            onClick={handleRegenerate}
className="text-(--font-size-micro-sm) text-muted-foreground hover:text-indigo-500 dark:hover:text-indigo-300 shrink-0"
                            title={t("ai.regenerate")}
                          >
                            <RotateCw className="w-4 h-4" />
                          </Button>
                        )}
                        {msg.role === "assistant" && !msg.thinking && (
                          <CopyButton text={msg.content} size="xs"
                            className="text-(--font-size-micro-sm) text-muted-foreground hover:text-indigo-500 dark:hover:text-indigo-300 shrink-0"
                            title={t("ai.copy")} onError={() => toast.error(t("ai.toast.copy_failed"))}>
                            {(copied) => (<>{copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />} {copied ? t("ai.copied") : t("ai.copy")}</>)}
                          </CopyButton>
                        )}
                      </div>
                    </div>
                    {msg.role === "tool" ? (
                      <pre className="text-xs text-muted-foreground whitespace-pre-wrap break-words font-mono leading-relaxed max-h-80 overflow-auto">
                        {msg.content}
                      </pre>
                    ) : msg.role === "assistant" ? (
                      <SanitizedMarkdown content={msg.content} />
                    ) : (
                      <p className={`text-sm whitespace-pre-wrap ${msg.role === "user" ? "" : "text-muted-foreground"}`}>
                        {msg.content}
                      </p>
                    )}
                  </div>
                </div>
              );
            })
          )}
          {loading && !messages.some((m) => m.thinking) && (
            <div className="flex gap-3">
              <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/40 rounded-xl flex items-center justify-center shrink-0">
                <Bot className="w-4 h-4" />
              </div>
              <Card className="rounded-tl-md px-4 py-3">
                <div className="flex gap-1">
                  <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce"></span>
                  <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce delay-100"></span>
                  <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce delay-200"></span>
                </div>
              </Card>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        <Card className="shrink-0 p-2 sm:p-3">
          <div className="flex items-end gap-2">
            <Textarea
              ref={textareaRef}
              rows={1}
              placeholder={t("ai.input_placeholder")}
              className="flex-1 resize-none border-0 focus:ring-0 text-sm py-2.5 px-2 max-h-32 bg-transparent text-foreground placeholder:text-muted-foreground outline-none"
              value={input}
              onChange={(e) => { setInput(e.target.value); adjustTextarea(); }}
              onKeyDown={handleKeyDown}
            />
            {loading ? (
              <Button variant="destructive" size="icon" onClick={handleStop} className="shrink-0 rounded-xl" aria-label="Stop generation">
                <Square className="w-4 h-4" />
              </Button>
            ) : (
              <Button size="icon" onClick={handleSend} className="shrink-0 rounded-xl" aria-label="Send message">
                <Send className="w-4 h-4" />
              </Button>
            )}
          </div>
          <div className="flex justify-between items-center mt-1.5 px-1">
            <span className="text-(--font-size-micro-sm) text-muted-foreground">
              {messages.filter((m) => !m.thinking).length}/40
              {input.trim() && <> &middot; ~{Math.ceil(input.trim().length / 4)} {t("ai.tokens_est")}</>}
            </span>
            <span className="text-(--font-size-micro-sm) text-muted-foreground hidden sm:inline">{t("ai.input_hint")}</span>
          </div>
        </Card>
      </div>

      <Dialog open={!!renameTarget} onOpenChange={() => setRenameTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("ai.rename") || "Rename conversation"}</DialogTitle>
          </DialogHeader>
          <Input
            type="text"
            value={renameValue}
            onChange={(e) => setRenameValue(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") confirmRename(); }}
            className="w-full"
            autoFocus
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenameTarget(null)}>Cancel</Button>
            <Button onClick={confirmRename}>Rename</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
