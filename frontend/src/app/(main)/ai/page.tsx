"use client";

import { useEffect, useState, useRef, useCallback } from "react";
import { useI18n } from "@/lib/i18n";
import { API_BASE } from "@/lib/constants";

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
}


export default function AIPage() {
  const { t } = useI18n();
  const [messages, setMessages] = useState<AIMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [configured, setConfigured] = useState(false);
  const [hasAPIKey, setHasAPIKey] = useState(false);
  const [provider, setProvider] = useState("deepseek");
  const [model, setModel] = useState("deepseek-chat");
  const [endpoint, setEndpoint] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [configSaving, setConfigSaving] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => { scrollToBottom(); }, [messages]);

  const loadConfig = useCallback(async () => {
    try {
      setSettingsLoading(true);
      const res = await fetch(`${API_BASE}?p=/ai&format=json`);
      if (res.ok) {
        const data = await res.json();
        setConfigured(Boolean(data.AIConfigured));
        setHasAPIKey(Boolean(data.AIHasAPIKey));
        if (data.AIConfig) {
          const cfg = data.AIConfig as AIConfig;
          if (cfg.provider) setProvider(cfg.provider);
          if (cfg.model) setModel(cfg.model);
          if (cfg.endpoint) setEndpoint(cfg.endpoint);
          if (cfg.system_prompt) setSystemPrompt(cfg.system_prompt);
        }
      }
    } catch (e) { console.error("AI: load config failed", e); } finally {
      setSettingsLoading(false);
    }
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadConfig()); }, [loadConfig]);

  const handleSaveConfig = async () => {
    try {
      setConfigSaving(true);
      const res = await fetch(`${API_BASE}?p=/ai/config`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: true, provider, model, endpoint, system_prompt: systemPrompt }),
        credentials: "include",
      });
      if (res.ok) {
        const data = await res.json();
        if (data.success) {
          setConfigured(true);
          setShowSettings(false);
        }
      }
    } catch (e) { console.error("AI: save config failed", e); } finally {
      setConfigSaving(false);
    }
  };

  const handleSend = async () => {
    if (!input.trim() || loading) return;
    const userMsg: AIMessage = { role: "user", content: input.trim() };

    const conversationHistory = [...messages, userMsg];
    setMessages((prev) => [...prev, userMsg]);
    setInput("");
    setLoading(true);

    const controller = new AbortController();
    abortRef.current = controller;

    const thinkingMsg: AIMessage = { role: "assistant", content: "", thinking: true };
    setMessages((prev) => [...prev, thinkingMsg]);

    try {
      const response = await fetch(`${API_BASE}?p=/ai/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ messages: conversationHistory }),
        credentials: "include",
        signal: controller.signal,
      });

      if (!response.ok || !response.body) {
        setMessages((prev) => {
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
                setMessages((prev) => {
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
                setMessages((prev) => {
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
                setMessages((prev) => {
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
                setMessages((prev) => {
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
                setMessages((prev) => [
                  ...prev,
                  { role: "tool", content: toolLabel, tool_name: data },
                ]);
                break;
              }
              case "tool": {
                try {
                  const parsed = JSON.parse(data);
                  const resultLabel = `${t("ai.tool_result")}:\n${parsed.result}`;
                  setMessages((prev) => {
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
                  setMessages((prev) => [
                    ...prev,
                    { role: "tool", content: data, tool_name: t("ai.tool") },
                  ]);
                }
                break;
              }
              case "error": {
                setMessages((prev) => [
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
      setMessages((prev) => {
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
    setMessages([]);
  };

  const handleExport = () => {
    const text = messages
      .filter((m) => !m.thinking)
      .map((m) => {
        const roleLabel = m.role === "user" ? "[user]" : m.role === "assistant" ? "[assistant]" : `[tool:${m.tool_name}]`;
        return `${roleLabel} ${m.content}`;
      })
      .join("\n\n");
    const blob = new Blob([text], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "ai-chat-export.txt";
    a.click();
    URL.revokeObjectURL(url);
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

  return (
    <div className="h-full min-h-0 flex flex-col max-w-5xl mx-auto w-full pb-20 md:pb-0">
      <div className="shrink-0 flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-3 px-1">
        <div className="min-w-0">
          <h1 className="text-xl sm:text-2xl font-semibold tracking-tight text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <span className="w-9 h-9 rounded-xl bg-indigo-100 dark:bg-indigo-900/40 flex items-center justify-center shrink-0">
              <i className="fa-solid fa-robot text-indigo-500 dark:text-indigo-400"></i>
            </span>
            {t("nav.ai")}
          </h1>
          <p className="text-slate-500 dark:text-slate-400 text-xs mt-1.5 flex items-center gap-2 flex-wrap">
            {configured ? (
              <>
                <span className="inline-flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
                  <span className="w-1.5 h-1.5 bg-emerald-500 rounded-full animate-pulse"></span>
                  {t("common.online")}
                </span>
                <span className="text-slate-400">&middot;</span>
                <span className="font-semibold text-indigo-600 dark:text-indigo-400">{provider}</span>
                {hasAPIKey && <span className="inline-flex items-center gap-1 text-[10px] text-emerald-600 dark:text-emerald-400"><i className="fa-solid fa-key"></i> API key set</span>}
                <span className="font-mono text-[10px] bg-slate-100 dark:bg-slate-700 px-1.5 py-0.5 rounded">{model}</span>
              </>
            ) : (
              <span className="inline-flex items-center gap-1.5 text-amber-600 dark:text-amber-400">
                <i className="fa-solid fa-circle-exclamation text-[10px]"></i>
                {t("ai.not_configured")}
              </span>
            )}
          </p>
        </div>
        <div className="flex items-center gap-2 flex-wrap shrink-0">
          <button onClick={() => setShowSettings(!showSettings)} className="ui-btn ui-btn-secondary h-9 text-xs">
            <i className="fa-solid fa-gear"></i> {t("common.edit")}
          </button>
          <button onClick={handleClear} className="ui-btn ui-btn-danger h-9 text-xs">
            <i className="fa-solid fa-broom"></i> {t("ai.clear")}
          </button>
          <button onClick={handleExport} className="ui-btn ui-btn-secondary h-9 text-xs">
            <i className="fa-solid fa-download"></i> {t("ai.export")}
          </button>
        </div>
      </div>

      {showSettings && (
        <div className="shrink-0 mb-3 ui-card p-4 sm:p-6 border border-[var(--border)]">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-semibold text-slate-900 dark:text-slate-100">{t("ai.config_title")}</h2>
            <button onClick={() => setShowSettings(false)} className="w-8 h-8 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-400 dark:text-slate-500 flex items-center justify-center">
              <i className="fa-solid fa-times"></i>
            </button>
          </div>
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <label className="flex items-center gap-2 cursor-pointer">
                <span className="w-2 h-2 rounded-full bg-emerald-500"></span>
                <span className="text-sm text-[var(--text-secondary)]">{t("ai.enable_ai")}</span>
              </label>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">{t("ai.provider")}</label>
                <select className="ui-select w-full" value={provider} onChange={(e) => setProvider(e.target.value)}>
                  <option value="deepseek">DeepSeek</option>
                  <option value="openai">OpenAI</option>
                  <option value="claude">Claude</option>
                  <option value="qianwen">Qianwen</option>
                  <option value="longcat">LongCat</option>
                  <option value="custom">Custom</option>
                </select>
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">{t("ai.model")}</label>
                <input type="text" value={model} onChange={(e) => setModel(e.target.value)} className="ui-input font-mono w-full" />
              </div>
              <div className="md:col-span-2">
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">{t("ai.endpoint") || "API Endpoint (optional)"}</label>
                <input type="text" value={endpoint} onChange={(e) => setEndpoint(e.target.value)} placeholder="https://api.openai.com/v1" className="ui-input font-mono w-full text-xs" />
              </div>
            </div>
            <div>
              <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">{t("ai.system_prompt") || "System Prompt (optional)"}</label>
              <textarea value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} rows={3} className="ui-input w-full text-xs resize-y" placeholder="You are a helpful C2 operations assistant..." />
            </div>
            <button type="button" onClick={handleSaveConfig} disabled={configSaving} className="w-full h-10 ui-btn ui-btn-primary">
              {configSaving ? <><i className="fa-solid fa-spinner fa-spin mr-2"></i>{t("common.saving")}</> : t("common.save")}
            </button>
          </div>
        </div>
      )}

      <div className="flex-1 min-h-0 overflow-y-auto space-y-4 mb-3 pr-1 scroll-smooth rounded-2xl border border-slate-200/80 dark:border-slate-700/80 bg-white/50 dark:bg-slate-800/30 p-3 sm:p-4">
        {messages.length === 0 ? (
          <div className="flex gap-3">
            <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/40 rounded-xl flex items-center justify-center flex-shrink-0 mt-1">
              <i className="fa-solid fa-robot text-indigo-500 dark:text-indigo-400 text-sm"></i>
            </div>
            <div className="ui-card px-4 py-3 max-w-[90%] border border-slate-100 dark:border-slate-700">
              <p className="text-sm text-[var(--text-secondary)] font-medium">{t("ai.greeting_title")}</p>
              <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">{t("ai.greeting_desc")}</p>
              <div className="flex flex-wrap gap-2 mt-3">
                {quickActions.map((q) => (
                  <button key={q.label} onClick={() => { setInput(q.query); }} className="ui-btn ui-btn-ghost text-xs h-8" data-query={q.query}>
                    {q.label}
                  </button>
                ))}
              </div>
            </div>
          </div>
        ) : (
          messages.map((msg, i) => {
            if (msg.thinking) {
              return (
                <div key={i} className="flex gap-3">
                  <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/40 rounded-xl flex items-center justify-center flex-shrink-0">
                    <i className="fa-solid fa-robot text-indigo-500 dark:text-indigo-400 text-sm"></i>
                  </div>
                  <div className="ui-card rounded-tl-md px-4 py-3">
                    <div className="flex items-center gap-2 text-sm text-slate-500 dark:text-slate-400">
                      <i className="fa-solid fa-brain text-indigo-400 animate-pulse"></i>
                      <span>{t("ai.thinking")}</span>
                      <span className="flex gap-1 ml-1">
                        <span className="w-1.5 h-1.5 bg-indigo-400 rounded-full animate-bounce"></span>
                        <span className="w-1.5 h-1.5 bg-indigo-400 rounded-full animate-bounce" style={{ animationDelay: "0.15s" }}></span>
                        <span className="w-1.5 h-1.5 bg-indigo-400 rounded-full animate-bounce" style={{ animationDelay: "0.3s" }}></span>
                      </span>
                    </div>
                  </div>
                </div>
              );
            }
            return (
              <div key={i} className={`flex gap-3 ${msg.role === "user" ? "flex-row-reverse" : ""}`}>
                <div className={`w-8 h-8 rounded-xl flex items-center justify-center flex-shrink-0 mt-1 ${msg.role === "user" ? "bg-slate-200 dark:bg-slate-600" : msg.role === "tool" ? "bg-amber-100 dark:bg-amber-900/40" : "bg-indigo-100 dark:bg-indigo-900/40"}`}>
                  <i className={`fa-solid text-sm ${msg.role === "user" ? "fa-user text-slate-500 dark:text-slate-300" : msg.role === "tool" ? "fa-wrench text-amber-500" : "fa-robot text-indigo-500 dark:text-indigo-400"}`}></i>
                </div>
                <div className={`max-w-[80%] ${msg.role === "user" ? "bg-indigo-600 text-white rounded-2xl rounded-tr-md" : "ui-card rounded-tl-md"} px-4 py-3`}>
                  {msg.tool_name && (
                    <div className="text-[10px] font-mono text-amber-600 dark:text-amber-400 mb-1 flex items-center gap-1">
                      <i className="fa-solid fa-terminal"></i>
                      {msg.tool_name}
                    </div>
                  )}
                  <p className={`text-sm whitespace-pre-wrap ${msg.role === "user" ? "" : "text-[var(--text-secondary)]"}`}>{msg.content}</p>
                </div>
              </div>
            );
          })
        )}
        {loading && !messages.some((m) => m.thinking) && (
          <div className="flex gap-3">
            <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/40 rounded-xl flex items-center justify-center flex-shrink-0">
              <i className="fa-solid fa-robot text-indigo-500 dark:text-indigo-400 text-sm"></i>
            </div>
            <div className="ui-card rounded-tl-md px-4 py-3">
              <div className="flex gap-1">
                <span className="w-2 h-2 bg-slate-300 dark:bg-slate-500 rounded-full animate-bounce"></span>
                <span className="w-2 h-2 bg-slate-300 dark:bg-slate-500 rounded-full animate-bounce" style={{ animationDelay: "0.1s" }}></span>
                <span className="w-2 h-2 bg-slate-300 dark:bg-slate-500 rounded-full animate-bounce" style={{ animationDelay: "0.2s" }}></span>
              </div>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      <div className="shrink-0 ui-card p-2 sm:p-3 border border-[var(--border)] shadow-sm">
        <div className="flex items-end gap-2">
          <textarea
            rows={1}
            placeholder={t("ai.input_placeholder")}
            className="flex-1 resize-none border-0 focus:ring-0 text-sm py-2.5 px-2 max-h-32 bg-transparent text-[var(--text-primary)] placeholder:text-slate-400 outline-none"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          {loading ? (
            <button onClick={handleStop} className="w-10 h-10 ui-btn ui-btn-danger flex items-center justify-center flex-shrink-0 rounded-xl">
              <i className="fa-solid fa-stop text-sm"></i>
            </button>
          ) : (
            <button onClick={handleSend} className="w-10 h-10 ui-btn ui-btn-primary flex items-center justify-center flex-shrink-0 rounded-xl">
              <i className="fa-solid fa-paper-plane text-sm"></i>
            </button>
          )}
        </div>
        <div className="flex justify-between items-center mt-1.5 px-1">
          <span className="text-[10px] text-slate-400 dark:text-slate-500">{messages.filter((m) => !m.thinking).length}/40</span>
          <span className="text-[10px] text-slate-400 dark:text-slate-500 hidden sm:inline">{t("ai.input_hint")}</span>
        </div>
      </div>
    </div>
  );
}
