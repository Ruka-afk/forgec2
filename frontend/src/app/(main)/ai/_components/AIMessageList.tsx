"use client";

import { useI18n } from "@/lib/i18n";
import { CopyButton } from "@/components/ui/copy-button";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Activity, Bot, Brain, Check, Copy, KeyRound, Radio, RotateCw, Search, Terminal, User, Wrench } from "lucide-react";
import type { AIMessage } from "./types";
import { SanitizedMarkdown } from "./SanitizedMarkdown";

interface AIMessageListProps {
  messages: AIMessage[];
  loading: boolean;
  lastAssistantIndex: number;
  quickActions: { label: string; query: string }[];
  onPickQuick: (query: string) => void;
  onRegenerate: () => void;
  messagesEndRef: React.RefObject<HTMLDivElement | null>;
}

export function AIMessageList({
  messages,
  loading,
  lastAssistantIndex,
  quickActions,
  onPickQuick,
  onRegenerate,
  messagesEndRef,
}: AIMessageListProps) {
  const { t } = useI18n();
  const quickIcons = [Activity, Radio, Search, KeyRound, User];

  return (
    <div data-ai-message-scroll className="min-h-0 flex-1 overflow-y-auto scroll-smooth px-3 py-5 sm:px-5 sm:py-7">
      <div className="mx-auto flex min-h-full w-full max-w-4xl flex-col gap-6">
        {messages.length === 0 ? (
          <div className="mx-auto flex w-full max-w-2xl flex-col items-center px-1 py-3 text-center sm:m-auto sm:py-12">
            <div className="mb-3 flex size-12 items-center justify-center rounded-2xl border border-primary/15 bg-primary/10 text-primary shadow-sm sm:mb-5 sm:size-14">
              <Bot className="size-5 sm:size-6" />
            </div>
            <h2 className="text-xl font-semibold tracking-tight text-foreground">{t("ai.greeting_title")}</h2>
            <p className="mt-1.5 max-w-xl text-sm leading-6 text-muted-foreground sm:mt-2">{t("ai.greeting_desc")}</p>
            <div className="mt-5 grid w-full grid-cols-1 gap-2 sm:mt-7 sm:grid-cols-2">
              {quickActions.map((q, index) => {
                const QuickIcon = quickIcons[index] ?? Activity;
                return (
                  <Button
                    key={q.label}
                    variant="outline"
                    onClick={() => onPickQuick(q.query)}
                    data-query={q.query}
                    className="h-auto min-h-12 justify-start gap-3 rounded-xl bg-card px-3.5 py-2 text-left font-normal shadow-xs hover:border-primary/30 hover:bg-primary/5 sm:py-3"
                  >
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                      <QuickIcon className="size-4" />
                    </span>
                    <span className="min-w-0 whitespace-normal leading-5">{q.label}</span>
                  </Button>
                );
              })}
            </div>
          </div>
        ) : (
          messages.map((msg, i) => {
            if (msg.thinking) {
              return (
                <div key={i} className="flex items-start gap-3">
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                    <Bot className="size-4" />
                  </div>
                  <div className="rounded-2xl rounded-tl-md border border-border/75 bg-card px-4 py-3 shadow-xs">
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Brain className="size-4 text-primary" />
                      <span>{t("ai.thinking")}</span>
                      <span className="ml-1 flex gap-1" aria-hidden="true">
                        <span className="size-1.5 animate-bounce rounded-full bg-primary" />
                        <span className="size-1.5 animate-bounce rounded-full bg-primary delay-150" />
                        <span className="size-1.5 animate-bounce rounded-full bg-primary delay-300" />
                      </span>
                    </div>
                  </div>
                </div>
              );
            }
            return (
              <div key={i} className={`group flex items-start gap-3 ${msg.role === "user" ? "flex-row-reverse" : ""}`}>
                <div className={`flex size-8 shrink-0 items-center justify-center rounded-xl ${msg.role === "user" ? "bg-foreground text-background" : msg.role === "tool" ? "bg-warning/15 text-warning" : "bg-primary/10 text-primary"}`}>
                  {msg.role === "user" ? <User className="size-4" /> : msg.role === "tool" ? <Wrench className="size-4" /> : <Bot className="size-4" />}
                </div>
                <div className={`min-w-0 px-4 py-3 ${msg.role === "user" ? "max-w-[85%] rounded-2xl rounded-tr-md bg-primary text-primary-foreground shadow-sm sm:max-w-[72%]" : msg.role === "tool" ? "max-w-[92%] rounded-2xl rounded-tl-md border border-warning/25 bg-warning/5 sm:max-w-[82%]" : "max-w-[92%] rounded-2xl rounded-tl-md border border-border/75 bg-card text-card-foreground shadow-xs sm:max-w-[82%]"}`}>
                  {msg.role !== "user" && (
                    <div className="mb-2 flex min-h-5 items-center justify-between gap-3">
                      <div className={`flex min-w-0 items-center gap-1.5 text-(--fs-micro-sm) font-medium ${msg.role === "tool" ? "font-mono text-warning" : "text-muted-foreground"}`}>
                        {msg.role === "tool" ? <Terminal className="size-3.5" /> : <Bot className="size-3.5 text-primary" />}
                        <span className="truncate">{msg.tool_name || t("nav.ai")}</span>
                      </div>
                      <div className="flex items-center gap-1 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100">
                        {msg.role === "assistant" && !msg.thinking && i === lastAssistantIndex && (
                          <Button variant="ghost" size="icon-xs" onClick={onRegenerate} title={t("ai.regenerate")} aria-label={t("ai.regenerate")}>
                            <RotateCw className="size-3.5" />
                          </Button>
                        )}
                        {msg.role === "assistant" && !msg.thinking && (
                          <CopyButton text={msg.content} size="icon-xs" className="text-muted-foreground hover:text-primary" title={t("ai.copy")} onError={() => toast.error(t("ai.toast.copy_failed"))}>
                            {(copied) => copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
                          </CopyButton>
                        )}
                      </div>
                    </div>
                  )}
                  {msg.role === "tool" ? (
                    <pre className="max-h-80 overflow-auto whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-muted-foreground">{msg.content}</pre>
                  ) : msg.role === "assistant" ? (
                    <SanitizedMarkdown content={msg.content} />
                  ) : (
                    <p className="whitespace-pre-wrap text-sm leading-6">{msg.content}</p>
                  )}
                </div>
              </div>
            );
          })
        )}
        {loading && !messages.some((m) => m.thinking) && (
          <div className="flex items-start gap-3">
            <div className="flex size-8 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary"><Bot className="size-4" /></div>
            <div className="rounded-2xl rounded-tl-md border border-border/75 bg-card px-4 py-3 shadow-xs">
              <div className="flex gap-1.5" aria-label={t("ai.thinking")}>
                <span className="size-1.5 animate-bounce rounded-full bg-muted-foreground" />
                <span className="size-1.5 animate-bounce rounded-full bg-muted-foreground delay-100" />
                <span className="size-1.5 animate-bounce rounded-full bg-muted-foreground delay-200" />
              </div>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>
    </div>
  );
}
