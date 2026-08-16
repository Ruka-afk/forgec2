"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { useI18n } from "@/lib/i18n";
import { API_BASE } from "@/lib/constants";
import { downloadText } from "@/lib/download";
import { api, getCsrfToken } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { toast } from "sonner";
import { Bot, Download, Menu, Settings, Wand2 } from "lucide-react";
import AISessionSidebar from "./_components/AISessionSidebar";
import type { AIMessage } from "./_components/types";
import { useAIConfig } from "./_components/useAIConfig";
import { useAISessions } from "./_components/useAISessions";
import { AIMessageList } from "./_components/AIMessageList";
import { AIComposer } from "./_components/AIComposer";
import { AIConfigPanel } from "./_components/AIConfigPanel";

export default function AIPage() {
  const { t } = useI18n();
  const [messages, setMessages] = useState<AIMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const {
    provider, setProvider, model, setModel, apiKey, setApiKey,
    endpoint, setEndpoint, systemPrompt, setSystemPrompt,
    allowExecute, setAllowExecute, configSaving, showSettings, setShowSettings,
    handleSaveConfig,
  } = useAIConfig();

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
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => { scrollToBottom(); }, [messages]);
  useEffect(() => { adjustTextarea(); }, [input]);
  useEffect(() => {
    return () => abortRef.current?.abort();
  }, []);

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
              aria-label={t("ai.toggle_sessions")}
            >
              <Menu className="w-4 h-4" />
            </Button>
            <PageHeader title={<><span className="w-9 h-9 rounded-lg bg-primary/10 text-primary flex items-center justify-center shrink-0 ring-1 ring-border/50"><Bot className="w-4 h-4" /></span>{t("nav.ai")}</>} />
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
          <span className="text-(--fs-xs-sm) text-muted-foreground">
            {activeSessionId != null ? `${t("ai.sessions")} #${activeSessionId}` : t("ai.new_chat")}
          </span>
        </div>

        {showSettings && (
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
            allowExecute={allowExecute}
            setAllowExecute={setAllowExecute}
            configSaving={configSaving}
            onClose={() => setShowSettings(false)}
            onSave={handleSaveConfig}
          />
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

        <AIComposer
          input={input}
          loading={loading}
          messageCount={messages.filter((m) => !m.thinking).length}
          textareaRef={textareaRef}
          onChange={(v) => { setInput(v); adjustTextarea(); }}
          onKeyDown={handleKeyDown}
          onSend={handleSend}
          onStop={handleStop}
        />
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
            <Button variant="outline" onClick={() => setRenameTarget(null)}>{t("common.cancel")}</Button>
            <Button onClick={confirmRename}>{t("common.rename")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
