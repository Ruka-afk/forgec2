"use client";

import { useEffect, useMemo, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { CopyButton } from "@/components/ui/copy-button";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Activity, Bell, BookOpen, Bot, Brain, Check, CheckCircle2, ChevronDown, CircleAlert, Clock3, Copy, Database, FileJson2, GitBranch, Lightbulb, LoaderCircle, Radio, RotateCw, Shield, User, Wrench } from "lucide-react";
import type { AIMessage } from "./types";
import { SanitizedMarkdown } from "./SanitizedMarkdown";
import { AITracePanel } from "./AITracePanel";
import { cn } from "@/lib/utils";
import { describeToolOutput, extractAICitations, formatAIRunDuration } from "./aiOutput";

interface AIMessageListProps {
  messages: AIMessage[];
  loading: boolean;
  lastAssistantIndex: number;
  quickActions: { label: string; query: string }[];
  followUps: { label: string; query: string }[];
  onFollowUp: (query: string) => void;
  onRegenerate: () => void;
  onBranch?: (messageId: number) => void;
  messagesEndRef: React.RefObject<HTMLDivElement | null>;
}

export function AIMessageList({
  messages,
  loading,
  lastAssistantIndex,
  quickActions,
  followUps,
  onFollowUp,
  onRegenerate,
  onBranch,
  messagesEndRef,
}: AIMessageListProps) {
  const { t } = useI18n();
  const quickIcons = [Activity, Radio, Shield, Bell, Lightbulb];
  const toolMessages = useMemo(() => messages.filter((m) => m.role === "tool"), [messages]);
  const [expandAll, setExpandAll] = useState<boolean | null>(null);

  return (
    <div
      data-ai-message-scroll
      role="log"
      aria-live="polite"
      aria-relevant="additions text"
      aria-busy={loading}
      className="min-h-0 flex-1 overflow-y-auto bg-background/40 px-3 py-5 sm:px-5 sm:py-7"
    >
      <div className="mx-auto flex min-h-full w-full max-w-5xl flex-col gap-5">
        {toolMessages.length > 1 && (
          <div className="flex flex-wrap items-center justify-end gap-1.5">
            <Button variant="ghost" size="xs" onClick={() => setExpandAll(true)}>{t("ai.tool_expand_all")}</Button>
            <Button variant="ghost" size="xs" onClick={() => setExpandAll(false)}>{t("ai.tool_collapse_all")}</Button>
            <Button variant="ghost" size="xs" onClick={() => { const all = toolMessages.map((m) => { try { return JSON.stringify(JSON.parse(m.content), null, 2); } catch { return m.content; } }).join("\n\n---\n\n"); navigator.clipboard.writeText(all).then(() => toast.success(t("ai.copied"))).catch(() => toast.error(t("ai.toast.copy_failed"))); }}>{t("ai.tool_copy_all")}</Button>
          </div>
        )}
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
                    onClick={() => onFollowUp(q.query)}
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
            if (msg.trace) {
              return (
                <AITracePanel
                  key={msg.stream_id || `trace-${i}`}
                  steps={msg.trace}
                  status={msg.trace_status || "complete"}
                  reasoning={msg.reasoning}
                />
              );
            }
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
            if (msg.role === "tool") {
              return (
                <ToolResultBlock
                  key={msg.stream_id ? `${msg.stream_id}-${msg.tool_call_id || "tool"}-${i}` : `tool-${i}`}
                  toolName={msg.tool_name || t("ai.tool")}
                  content={msg.content}
                  status={msg.tool_status}
                  expandAll={expandAll}
                />
              );
            }
            return (
              <div key={msg.stream_id ? `${msg.stream_id}-${msg.tool_call_id || msg.role}-${i}` : i} className={`group flex items-start gap-3 ${msg.role === "user" ? "flex-row-reverse" : ""}`}>
                <div className={`flex size-8 shrink-0 items-center justify-center rounded-xl shadow-xs ring-1 ${msg.role === "user" ? "bg-foreground text-background ring-foreground/10" : msg.error ? "bg-destructive/10 text-destructive ring-destructive/20" : "bg-primary/10 text-primary ring-primary/15"}`}>
                  {msg.role === "user" ? <User className="size-4" /> : <Bot className="size-4" />}
                </div>
                <div className={`min-w-0 ${msg.role === "user" ? "max-w-[85%] rounded-2xl rounded-tr-md bg-primary px-4 py-3 text-primary-foreground shadow-sm sm:max-w-[72%]" : msg.error ? "max-w-[94%] flex-1 rounded-2xl rounded-tl-md border border-destructive/25 border-l-2 border-l-destructive/60 bg-card px-4 py-4 text-card-foreground shadow-sm sm:px-5" : "max-w-[94%] flex-1 rounded-2xl rounded-tl-md border border-border/80 border-l-2 border-l-primary/45 bg-card px-4 py-4 text-card-foreground shadow-sm sm:px-5"}`}>
                  {msg.role !== "user" && (
                    <div className="mb-3 flex min-h-6 items-center justify-between gap-3 border-b border-border/55 pb-2.5">
                      <div className={`flex min-w-0 items-center gap-1.5 text-xs font-semibold ${msg.error ? "text-destructive" : "text-foreground"}`}>
                        {msg.error ? <CircleAlert className="size-3.5 text-destructive" /> : <Bot className="size-3.5 text-primary" />}
                        <span className="truncate">{t("nav.ai")}</span>
                      </div>
                      <div className="flex items-center gap-0.5 rounded-lg bg-muted/60 p-0.5 opacity-100 transition-opacity sm:opacity-60 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100">
                        {msg.role === "assistant" && !msg.thinking && i === lastAssistantIndex && !loading && (
                          <Button variant="ghost" size="icon-xs" onClick={onRegenerate} title={msg.error ? t("ai.retry") : t("ai.regenerate")} aria-label={msg.error ? t("ai.retry") : t("ai.regenerate")}>
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
                  {msg.role === "assistant" ? (
                    <>
                      <SanitizedMarkdown content={msg.content} live={loading && i === lastAssistantIndex} />
                      <AssistantOutputMeta message={msg} />
                    </>
                  ) : (
                    <p className="whitespace-pre-wrap text-sm leading-6">{msg.content}</p>
                  )}
                  {msg.role === "user" && msg.id && onBranch && !loading && (
                    <div className="mt-2 flex justify-end opacity-80 transition-opacity sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100">
                      <Button variant="ghost" size="xs" className="h-7 text-primary-foreground/80 hover:bg-white/10 hover:text-primary-foreground" onClick={() => onBranch(msg.id!)} title={t("ai.branch_from_here")}>
                        <GitBranch className="size-3" />{t("ai.branch_from_here")}
                      </Button>
                    </div>
                  )}
                  {msg.role === "assistant" && !msg.thinking && i === lastAssistantIndex && !loading && followUps.length > 0 && (
                    <div className="mt-4 flex flex-wrap gap-1.5 border-t border-border/55 pt-3">
                      {followUps.map((item) => (
                        <Button
                          key={item.query}
                          type="button"
                          variant="outline"
                          size="xs"
                          onClick={() => onFollowUp(item.query)}
                          className="h-7 rounded-full px-2.5 text-xs font-normal"
                        >
                          {item.label}
                        </Button>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            );
          })
        )}
        {loading && !messages.some((m) => m.thinking || m.trace_status === "running") && (
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

function AssistantOutputMeta({ message }: { message: AIMessage }) {
  const { t } = useI18n();
  const citations = extractAICitations(message.content);
  const terminal = message.run_status && message.run_status !== "streaming";
  if (!terminal && citations.length === 0 && !message.created_at) return null;
  const tokens = (message.prompt_tokens || 0) + (message.completion_tokens || 0);
  const duration = formatAIRunDuration(message.duration_ms);
  const unsuccessful = message.run_status === "error" || message.run_status === "interrupted";
  const completedAt = message.created_at ? new Date(message.created_at) : null;
  return (
    <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-border/60 pt-2 text-(--fs-micro-sm) text-muted-foreground">
      {terminal && (
        <span className={cn("inline-flex items-center gap-1", unsuccessful ? "text-destructive" : "text-success")}>
          {unsuccessful ? <CircleAlert className="size-3" /> : <CheckCircle2 className="size-3" />}
          {message.run_status === "interrupted" ? t("ai.output_interrupted") : message.run_status === "error" ? t("ai.output_error") : t("ai.output_complete")}
        </span>
      )}
      {completedAt && !Number.isNaN(completedAt.getTime()) && <time dateTime={message.created_at} title={completedAt.toLocaleString()}>{completedAt.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</time>}
      {duration && <span className="inline-flex items-center gap-1"><Clock3 className="size-3" />{duration}</span>}
      {tokens > 0 && <span>{tokens.toLocaleString()} {t("ai.output_tokens")}</span>}
      {message.truncated && <span className="inline-flex items-center gap-1 text-warning"><CircleAlert className="size-3" />{t("ai.output_truncated")}</span>}
      {citations.length > 0 && (
        <span className="flex basis-full flex-wrap items-center gap-1.5 pt-1">
          <span className="inline-flex items-center gap-1"><BookOpen className="size-3" />{t("ai.output_sources")}</span>
          {citations.map((citation) => <span key={citation} className="max-w-full truncate rounded-md border border-border/70 bg-muted/70 px-1.5 py-0.5 font-mono" title={citation}>{citation}</span>)}
        </span>
      )}
    </div>
  );
}

function ToolResultBlock({ toolName, content, status = "success", expandAll }: { toolName: string; content: string; status?: AIMessage["tool_status"]; expandAll?: boolean | null }) {
  const { t } = useI18n();
  const view = useMemo(() => describeToolOutput(content, status), [content, status]);
  const [open, setOpen] = useState(false);
  useEffect(() => { if (expandAll !== null) setOpen(expandAll as boolean); }, [expandAll]);
  const summary = useMemo(() => {
    if (!view.raw) return "";
    if (view.isJson) {
      try {
        const parsed = JSON.parse(view.raw) as Record<string, unknown>;
        const message = typeof parsed.error === "string"
          ? parsed.error
          : typeof parsed.message === "string"
            ? parsed.message
            : "";
        if (message) return message;
      } catch {
        // The formatter already validated JSON; retain a safe fallback.
      }
    }
    const firstLine = view.raw.split(/\r?\n/, 1)[0].trim();
    return firstLine.length > 120 ? `${firstLine.slice(0, 120)}…` : firstLine;
  }, [view]);
  const statusLabel = view.status === "running"
    ? t("ai.tool_running")
    : view.status === "waiting_approval"
      ? t("ai.tool_waiting_approval")
      : view.status === "error"
        ? t("ai.tool_failed")
        : t("ai.tool_succeeded");
  const StatusIcon = view.status === "running" ? LoaderCircle : view.status === "error" ? CircleAlert : view.status === "waiting_approval" ? Shield : CheckCircle2;
  return (
    <div className={cn(
      "-my-1 overflow-hidden rounded-xl border bg-muted/25 shadow-xs sm:ml-11",
      view.status === "error" && "border-destructive/25 bg-destructive/4",
      view.status === "waiting_approval" && "border-warning/30 bg-warning/4",
    )}>
      <div className="flex min-h-10 items-center gap-1 px-1.5">
        <button
          type="button"
          onClick={() => setOpen((value) => !value)}
          aria-expanded={open}
          aria-label={open ? t("ai.tool_collapse") : t("ai.tool_expand")}
          title={open ? t("ai.tool_collapse") : t("ai.tool_expand")}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-2 py-1.5 text-left hover:bg-muted/55"
        >
          <span className={cn(
            "flex size-6 shrink-0 items-center justify-center rounded-md",
            view.status === "error" ? "bg-destructive/10 text-destructive" : view.status === "success" ? "bg-success/10 text-success" : "bg-warning/10 text-warning",
          )}>
            <StatusIcon className={cn("size-3.5", view.status === "running" && "animate-spin motion-reduce:animate-none")} />
          </span>
          <Wrench className="size-3.5 shrink-0 text-muted-foreground" />
          <span className="shrink-0 font-mono text-xs font-semibold text-foreground">{toolName}</span>
          <span className={cn("hidden shrink-0 text-(--fs-micro-sm) sm:inline", view.status === "error" ? "text-destructive" : view.status === "success" ? "text-success" : "text-warning")}>{statusLabel}</span>
          {view.itemCount != null && <span className="hidden shrink-0 items-center gap-1 text-(--fs-micro-sm) text-muted-foreground md:inline-flex"><Database className="size-3" />{t("ai.tool_records", { count: view.itemCount })}</span>}
          {view.partial && <span className="shrink-0 text-(--fs-micro-sm) text-warning">{t("ai.tool_partial")}</span>}
          {summary && <span className={cn("min-w-0 flex-1 truncate text-xs text-muted-foreground", view.status === "error" && "text-destructive/85")}>· {summary}</span>}
          <ChevronDown className={cn("size-3.5 shrink-0 text-muted-foreground transition-transform", open && "rotate-180")} />
        </button>
        {view.raw && <CopyButton text={view.raw} size="icon-xs" className="shrink-0 text-muted-foreground" title={t("ai.copy")} onError={() => toast.error(t("ai.toast.copy_failed"))} />}
      </div>
      {open && view.formatted && (
        <div className="border-t border-border/65 px-3 py-2.5">
          <div className="mb-2 flex flex-wrap items-center gap-2 text-(--fs-micro-sm) text-muted-foreground">
            {view.isJson && <span className="inline-flex items-center gap-1"><FileJson2 className="size-3" />JSON</span>}
            {view.lineCount > 1 && <span>{t("ai.tool_lines", { count: view.lineCount })}</span>}
            {view.byteCount > 1024 && <span>{(view.byteCount / 1024).toFixed(1)}KB</span>}
            {view.originalBytes != null && <span title={t("ai.tool_original_size")}>{(view.originalBytes / 1024).toFixed(1)}KB {t("ai.tool_original")}</span>}
          </div>
          <pre className={cn("max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-lg border px-3 py-2 font-mono text-xs leading-relaxed", view.status === "error" ? "border-destructive/20 bg-destructive/5 text-destructive" : "border-border/70 bg-background/70 text-muted-foreground")}>
            {view.formatted}
          </pre>
        </div>
      )}
    </div>
  );
}
