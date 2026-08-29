"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import { useI18n } from "@/lib/i18n";
import { API_BASE } from "@/lib/constants";
import { downloadText } from "@/lib/download";
import { api, getCsrfToken } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { WorkspaceShell } from "@/components/ui/workspace-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Bot, Download, Menu, Plus, Settings, Trash2 } from "lucide-react";
import AISessionSidebar from "./_components/AISessionSidebar";
import type { AIMessage } from "./_components/types";
import { useAIConfig } from "./_components/useAIConfig";
import { useAISessions } from "./_components/useAISessions";
import { AIMessageList } from "./_components/AIMessageList";
import { AIComposer } from "./_components/AIComposer";
import { AIConfigPanel } from "./_components/AIConfigPanel";
import { PendingAITasks } from "./_components/PendingAITasks";

export default function AIPage() {
  const { t } = useI18n();
  const [messages, setMessages] = useState<AIMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [usage, setUsage] = useState<{ prompt: number; completion: number }>({ prompt: 0, completion: 0 });
  const abortRef = useRef<AbortController | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const {
    provider, setProvider, model, setModel, apiKey, setApiKey,
    endpoint, setEndpoint, systemPrompt, setSystemPrompt,
    engagementNotes, setEngagementNotes,
    allowExecute, setAllowExecute, configSaving, showSettings, setShowSettings,
    handleSaveConfig, hasApiKey,
  } = useAIConfig();

  const adjustTextarea = () => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 128) + "px";
  };

  // Operator context: deep links like /ai?agent=<id>&q=<text> (e.g. the
  // "Ask AI" button on an agent page) focus the conversation on one agent.
  // Every chat request carries it so tools can resolve "this machine".
  const searchParams = useSearchParams();
  const [contextAgentId, setContextAgentId] = useState("");
  const deepLinkDoneRef = useRef(false);
  const handleSendRef = useRef<(textOverride?: string) => Promise<void>>(async () => {});

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

  const {
    sessions,
    setSessions,
    activeSessionId,
    setActiveSessionId,
    sidebarOpen,
    setSidebarOpen,
    renameTarget,
    setRenameTarget,
    renameValue,
    setRenameValue,
    loadSessions,
    selectSession,
    deleteSession,
    handleNewChat,
    renameSession,
  } = useAISessions(setMessagesBoth);

  const scrollToBottom = () => {
    const reducedMotion = typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    messagesEndRef.current?.scrollIntoView({ behavior: reducedMotion ? "auto" : "smooth" });
  };

  useEffect(() => {
    if (messages.length > 0) {
      scrollToBottom();
      return;
    }
    const viewport = messagesEndRef.current?.closest<HTMLElement>("[data-ai-message-scroll]");
    viewport?.scrollTo({ top: 0, behavior: "auto" });
  }, [messages]);
  useEffect(() => { adjustTextarea(); }, [input]);
  useEffect(() => {
    return () => abortRef.current?.abort();
  }, []);

  // Consume deep-link params once on mount: set context agent, prefill the
  // composer with q and auto-send it. No effect cleanup on purpose — under
  // StrictMode's double-invoke a returned clearTimeout would permanently
  // cancel the one-shot send; double-fire is guarded by deepLinkDoneRef.
  useEffect(() => {
    if (deepLinkDoneRef.current) return;
    const agent = searchParams.get("agent") || "";
    const q = searchParams.get("q") || "";
    if (!agent && !q) return;
    deepLinkDoneRef.current = true;
    if (agent) setContextAgentId(agent);
    if (q) {
      setInput(q);
      setTimeout(() => { void handleSendRef.current(q); }, 500);
    }
  }, [searchParams]);

  const confirmRename = async () => {
    if (!renameTarget) return;
    const title = renameValue.trim();
    if (title === "" || title === renameTarget.current) { setRenameTarget(null); return; }
    try {
      await api.putJson(paths.ai.session(renameTarget.id), { title });
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
    void handleSend(lastUserMsg.content);
  };

  // saveTurn persists every message added since `fromIndex` into the session.
  const saveTurn = async (sessionId: number, fromIndex: number) => {
    const msgs = messagesRef.current.slice(fromIndex);
    for (const m of msgs) {
      if (m.thinking) continue;
      try {
        await api.postJson(paths.ai.sessionMessages(sessionId), {
          role: m.role,
          content: m.content,
          tool_name: m.tool_name || "",
        });
      } catch { toast.error(t("ai.toast.save_message_failed")); }
    }
  };

  const handleSend = async (textOverride?: string) => {
    const text = (textOverride ?? input).trim();
    if (!text || loading) return;
    const userMsg: AIMessage = { role: "user", content: text };
    const userIndex = messagesRef.current.length;
    // Snapshot BEFORE dispatching: setMessagesBoth's updater mutates
    // messagesRef.current synchronously (React eager-evaluates it), so
    // spreading the ref again below would duplicate the user turn in the
    // LLM payload.
    const historyBefore = messagesRef.current.slice();
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
      const conversationHistory = [...historyBefore, userMsg];
      const response = await fetch(`${API_BASE}/ai/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": getCsrfToken() },
        body: JSON.stringify({
          messages: conversationHistory,
          context: {
            page: typeof window !== "undefined" ? window.location.pathname : "",
            agent_id: contextAgentId,
          },
        }),
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
              case "usage": {
                try {
                  const parsed = JSON.parse(data) as { prompt_tokens?: number; completion_tokens?: number };
                  setUsage((prev) => ({
                    prompt: prev.prompt + (parsed.prompt_tokens || 0),
                    completion: prev.completion + (parsed.completion_tokens || 0),
                  }));
                } catch { /* ignore malformed usage */ }
                break;
              }
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
  // Keep the ref in sync from an effect (not render body) so concurrent
  // renders never publish a half-built closure to the deep-link timer.
  useEffect(() => {
    handleSendRef.current = handleSend;
  });

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
    setUsage({ prompt: 0, completion: 0 });
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
    { label: t("ai.quick_situation"), query: t("ai.quick_situation_query") },
    { label: t("ai.quick_list_implants"), query: t("ai.quick_list_implants") },
    { label: t("ai.quick_list_listeners"), query: t("ai.quick_list_listeners") },
    { label: t("ai.quick_credential_summary"), query: t("ai.quick_credential_summary") },
    { label: t("ai.quick_who_online"), query: t("ai.quick_who_online") },
  ];

  const handleTaskFeedback = useCallback((content: string) => {
    setMessagesBoth((prev) => [...prev, { role: "user", content }]);
  }, [setMessagesBoth]);

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
    <WorkspaceShell
      className="h-full"
      sidebar={
        <AISessionSidebar
          sessions={sessions}
          activeSessionId={activeSessionId}
          onSelect={selectSession}
          onDelete={deleteSession}
          onRename={renameSession}
          onNewChat={handleNewChat}
        />
      }
    >

      {/* Mobile session sidebar */}
      <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
        <SheetContent side="left" className="w-[min(20rem,88vw)] p-0">
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

      {/* Configuration is secondary workspace chrome. Keeping it in a sheet
          preserves the conversation width and scroll position while editing. */}
      <Sheet open={showSettings} onOpenChange={setShowSettings}>
        <SheetContent side="right" className="w-full overflow-y-auto p-0 sm:max-w-xl">
          <AIConfigPanel
            provider={provider}
            setProvider={setProvider}
            model={model}
            setModel={setModel}
            apiKey={apiKey}
            setApiKey={setApiKey}
            endpoint={endpoint}
            setEndpoint={setEndpoint}
            systemPrompt={systemPrompt}
            setSystemPrompt={setSystemPrompt}
            engagementNotes={engagementNotes}
            setEngagementNotes={setEngagementNotes}
            allowExecute={allowExecute}
            setAllowExecute={setAllowExecute}
            configSaving={configSaving}
            onClose={() => setShowSettings(false)}
            onSave={handleSaveConfig}
          />
        </SheetContent>
      </Sheet>

      <div className="flex h-full min-h-0 min-w-0 w-full flex-1 flex-col bg-muted/20">
        <header className="flex min-h-16 shrink-0 items-center justify-between gap-3 border-b border-border/75 bg-card px-3 py-2.5 sm:px-5">
          <div className="flex min-w-0 items-center gap-3">
            <Button
              variant="outline"
              size="icon"
              onClick={() => setSidebarOpen(true)}
              className="lg:hidden"
              aria-label={t("ai.toggle_sessions")}
            >
              <Menu className="size-4" />
            </Button>
            <div className="icon-well size-9 border border-primary/15 bg-primary/10 text-primary sm:size-10">
              <Bot className="size-4" />
            </div>
            <div className="min-w-0">
              <div className="flex min-w-0 items-center gap-2">
                <h1 className="truncate text-sm font-semibold text-foreground sm:text-base">{t("nav.ai")}</h1>
                <Badge variant={hasApiKey ? "success" : "warning"} className="hidden max-w-48 truncate font-mono text-(--fs-micro-sm) sm:inline-flex">
                  {model || provider}
                </Badge>
              </div>
              <p className="truncate text-xs text-muted-foreground">
                {activeSessionId != null ? `${t("ai.sessions")} #${activeSessionId}` : t("ai.new_chat")}
              </p>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-1.5">
            <Button variant="outline" size="icon" onClick={handleNewChat} className="lg:hidden" aria-label={t("ai.new_chat")} title={t("ai.new_chat")}>
              <Plus className="size-4" />
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setShowSettings(true)} aria-label={t("ai.config_title")} title={t("ai.config_title")}>
              <Settings className="size-4" /> <span className="hidden sm:inline">{t("common.edit")}</span>
            </Button>
            <Button variant="ghost" size="sm" onClick={handleExport} disabled={messages.length === 0} aria-label={t("ai.export")} title={t("ai.export")}>
              <Download className="size-4" /> <span className="hidden sm:inline">{t("ai.export")}</span>
            </Button>
            <Button variant="ghost" size="sm" onClick={handleClear} disabled={messages.length === 0} className="text-muted-foreground hover:text-destructive" aria-label={t("ai.clear")} title={t("ai.clear")}>
              <Trash2 className="size-4" /> <span className="hidden sm:inline">{t("ai.clear")}</span>
            </Button>
          </div>
        </header>

        {!hasApiKey && (
          <div className="shrink-0 border-b border-warning/25 bg-warning/8 px-3 py-2.5 sm:px-5">
            <div className="mx-auto flex w-full max-w-4xl flex-col gap-1 text-sm text-warning sm:flex-row sm:items-center sm:justify-between">
              <span>{t("ai.no_api_key_warning")}</span>
              <Button variant="ghost" size="xs" onClick={() => setShowSettings(true)} className="h-7 self-start text-warning sm:self-auto">
                {t("ai.configure_now")}
              </Button>
            </div>
          </div>
        )}

        <AIMessageList
          messages={messages}
          loading={loading}
          lastAssistantIndex={lastAssistantIndex}
          quickActions={quickActions}
          onPickQuick={setInput}
          onRegenerate={handleRegenerate}
          messagesEndRef={messagesEndRef}
        />

        <div className="mx-auto w-full max-w-4xl shrink-0 px-3 sm:px-5">
          <PendingAITasks onTaskFeedback={handleTaskFeedback} />
        </div>

        {contextAgentId && (
          <div className="mx-auto flex w-full max-w-4xl items-center gap-2 px-4 pb-1.5 text-xs text-muted-foreground sm:px-5">
            <span>{t("ai.context_target")}</span>
            <Badge variant="info" className="font-mono">{contextAgentId.slice(0, 16)}</Badge>
            <button
              type="button"
              onClick={() => setContextAgentId("")}
              className="underline underline-offset-2 hover:text-foreground"
            >
              {t("common.clear")}
            </button>
          </div>
        )}

        <div className="shrink-0 border-t border-border/70 bg-card/95 px-3 py-3 sm:px-5 sm:py-4">
          <div className="mx-auto w-full max-w-4xl">
            <AIComposer
              input={input}
              loading={loading}
              disabled={!hasApiKey}
              messageCount={messages.filter((m) => !m.thinking).length}
              usage={usage}
              textareaRef={textareaRef}
              onChange={(v) => { setInput(v); adjustTextarea(); }}
              onKeyDown={handleKeyDown}
              onSend={handleSend}
              onStop={handleStop}
            />
          </div>
        </div>
      </div>

      <Dialog open={!!renameTarget} onOpenChange={() => setRenameTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("ai.rename")}</DialogTitle>
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
            <Button variant="outline" onClick={() => setRenameTarget(null)}>{t("common.cancel")}</Button>
            <Button onClick={confirmRename}>{t("common.rename")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </WorkspaceShell>
  );
}
