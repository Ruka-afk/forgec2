"use client";

import { useI18n } from "@/lib/i18n";
import { CopyButton } from "@/components/ui/copy-button";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Bot, Brain, Check, Copy, RotateCw, Terminal, User, Wrench } from "lucide-react";
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

  return (
    <div className="flex-1 min-h-0 overflow-y-auto space-y-4 mb-3 pr-1 scroll-smooth rounded-xl border border-border bg-card/50 p-3 sm:p-4">
      {messages.length === 0 ? (
        <div className="flex gap-3">
          <div className="w-8 h-8 bg-primary/10 dark:bg-primary/25 rounded-xl flex items-center justify-center shrink-0 mt-1">
            <Bot className="w-4 h-4" />
          </div>
          <Card className="px-4 py-3 max-w-[90%] border border-border">
            <p className="text-sm text-muted-foreground font-medium">{t("ai.greeting_title")}</p>
            <p className="text-sm text-muted-foreground mt-1">{t("ai.greeting_desc")}</p>
            <div className="flex flex-wrap gap-2 mt-3">
              {quickActions.map((q) => (
                <Button key={q.label} variant="ghost" size="xs" onClick={() => onPickQuick(q.query)} data-query={q.query}>
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
                <div className="w-8 h-8 bg-primary/10 dark:bg-primary/25 rounded-xl flex items-center justify-center shrink-0">
                  <Bot className="w-4 h-4" />
                </div>
                <Card className="rounded-tl-md px-4 py-3">
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Brain className="w-4 h-4" />
                    <span>{t("ai.thinking")}</span>
                    <span className="flex gap-1 ml-1">
                      <span className="w-1.5 h-1.5 bg-primary rounded-full animate-bounce" />
                      <span className="w-1.5 h-1.5 bg-primary rounded-full animate-bounce delay-150" />
                      <span className="w-1.5 h-1.5 bg-primary rounded-full animate-bounce delay-300" />
                    </span>
                  </div>
                </Card>
              </div>
            );
          }
          return (
            <div key={i} className={`flex gap-3 ${msg.role === "user" ? "flex-row-reverse" : ""}`}>
              <div className={`w-8 h-8 rounded-xl flex items-center justify-center shrink-0 mt-1 ${msg.role === "user" ? "bg-secondary" : msg.role === "tool" ? "bg-warning/15" : "bg-primary/10 dark:bg-primary/25"}`}>
                {msg.role === "user" ? <User className="w-4 h-4 text-muted-foreground" /> : msg.role === "tool" ? <Wrench className="w-4 h-4 text-warning" /> : <Bot className="w-4 h-4 text-primary" />}
              </div>
              <div className={`max-w-[80%] ${msg.role === "user" ? "bg-primary text-primary-foreground rounded-2xl rounded-tr-md" : "bg-card text-card-foreground ring-1 ring-foreground/10 rounded-tl-md"} px-4 py-3`}>
                <div className="flex items-center justify-between gap-2 mb-1">
                  {msg.tool_name ? (
                    <div className="text-(--fs-micro-sm) font-mono text-warning flex items-center gap-1">
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
                        onClick={onRegenerate}
                        className="text-(--fs-micro-sm) text-muted-foreground hover:text-primary shrink-0"
                        title={t("ai.regenerate")}
                      >
                        <RotateCw className="w-4 h-4" />
                      </Button>
                    )}
                    {msg.role === "assistant" && !msg.thinking && (
                      <CopyButton text={msg.content} size="xs"
                        className="text-(--fs-micro-sm) text-muted-foreground hover:text-primary shrink-0"
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
          <div className="w-8 h-8 bg-primary/10 dark:bg-primary/25 rounded-xl flex items-center justify-center shrink-0">
            <Bot className="w-4 h-4" />
          </div>
          <Card className="rounded-tl-md px-4 py-3">
            <div className="flex gap-1">
              <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" />
              <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce delay-100" />
              <span className="w-2 h-2 bg-muted-foreground rounded-full animate-bounce delay-200" />
            </div>
          </Card>
        </div>
      )}
      <div ref={messagesEndRef} />
    </div>
  );
}
