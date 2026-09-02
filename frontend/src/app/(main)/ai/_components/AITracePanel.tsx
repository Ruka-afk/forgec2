"use client";

import { useEffect, useRef, useState } from "react";
import {
  Brain,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  LoaderCircle,
  MessageSquareText,
  Search,
  Sparkles,
  Wrench,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import type { AITraceStage, AITraceStatus, AITraceStep } from "./types";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";

interface AITracePanelProps {
  steps: AITraceStep[];
  status: AITraceStatus;
  reasoning?: string;
}

const stageIcons: Record<AITraceStage, typeof Brain> = {
  analyzing: Search,
  reasoning: Brain,
  tool: Wrench,
  synthesizing: Sparkles,
  answering: MessageSquareText,
};

export function AITracePanel({ steps, status, reasoning }: AITracePanelProps) {
  const { t } = useI18n();
  const answering = steps.some((step) => step.stage === "answering");
  const [open, setOpen] = useState(status === "running" && !answering);
  const [now, setNow] = useState(() => Date.now());
  const reasoningRef = useRef<HTMLPreElement>(null);
  const userTouchedRef = useRef(false);

  useEffect(() => {
    if (userTouchedRef.current) return;
    setOpen(status === "running" && !answering);
  }, [status, answering]);

  useVisibleInterval(() => setNow(Date.now()), status === "running" ? 1000 : 0);

  useEffect(() => {
    if (status !== "running" || !reasoningRef.current) return;
    reasoningRef.current.scrollTop = reasoningRef.current.scrollHeight;
  }, [reasoning, status]);

  const stageLabel = (step: AITraceStep) => {
    switch (step.stage) {
      case "analyzing":
        return t("ai.trace.stage_analyzing");
      case "reasoning":
        return t("ai.trace.stage_reasoning");
      case "tool":
        return t("ai.trace.stage_tool", { tool: step.tool_name || t("ai.tool") });
      case "synthesizing":
        return t("ai.trace.stage_synthesizing");
      case "answering":
        return t("ai.trace.stage_answering");
    }
  };
  let liveStep = steps[steps.length - 1];
  for (let i = steps.length - 1; i >= 0; i--) {
    if (steps[i].status === "active") {
      liveStep = steps[i];
      break;
    }
  }
  const statusLabel = status === "running"
    ? t("ai.trace.running")
    : status === "complete"
      ? t("ai.trace.complete")
      : t("ai.trace.failed");

  return (
    <div className="flex items-start gap-3">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary shadow-xs ring-1 ring-primary/15">
        <Brain className="size-4" />
      </div>
      <Collapsible
        open={open}
        onOpenChange={(next) => {
          userTouchedRef.current = true;
          setOpen(next);
        }}
        className="min-w-0 max-w-[94%] flex-1 rounded-2xl rounded-tl-md border border-primary/15 border-l-2 border-l-primary/45 bg-card shadow-sm"
      >
        <CollapsibleTrigger className="flex min-h-12 w-full items-center justify-between gap-3 px-4 py-3 text-left hover:bg-muted/35 sm:px-5">
          <span className="flex min-w-0 items-center gap-2">
            {status === "running" ? (
              <LoaderCircle className="size-4 shrink-0 animate-spin text-primary motion-reduce:animate-none" />
            ) : status === "complete" ? (
              <CheckCircle2 className="size-4 shrink-0 text-success" />
            ) : (
              <CircleAlert className="size-4 shrink-0 text-destructive" />
            )}
            <span className="min-w-0">
              <span className="block truncate text-sm font-medium text-foreground">
                {status === "running"
                  ? (liveStep ? stageLabel(liveStep) : t("ai.trace.title"))
                  : t("ai.trace.title")}
              </span>
              <span className="block text-xs text-muted-foreground">
                {status === "running"
                  ? t("ai.trace.running")
                  : `${statusLabel} · ${steps.length} ${t("ai.trace.steps")}${reasoning ? ` · ${t("ai.trace.thinking")}` : ""}`}
              </span>
            </span>
          </span>
          <ChevronDown className={cn("size-4 shrink-0 text-muted-foreground transition-transform", open && "rotate-180")} />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <ol className="border-t border-border/70 bg-muted/20 px-4 py-4 sm:px-5">
            {steps.map((step, index) => {
              const Icon = stageIcons[step.stage];
              const duration = step.completed_at
                ? Math.max(0, step.completed_at - step.started_at)
                : Math.max(0, now - step.started_at);
              return (
                <li key={step.id} className="relative flex gap-3 pb-3 last:pb-0">
                  {index < steps.length - 1 && <span className="absolute left-[0.4375rem] top-5 h-[calc(100%-0.5rem)] w-px bg-border" />}
                  <span className={cn(
                    "relative z-10 mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-full border bg-card",
                    step.status === "active" && "border-primary text-primary",
                    step.status === "complete" && "border-success/60 text-success",
                    step.status === "error" && "border-destructive/60 text-destructive",
                  )}>
                    {step.status === "active" ? <LoaderCircle className="size-2.5 animate-spin motion-reduce:animate-none" /> : <Icon className="size-2.5" />}
                  </span>
                  <span className="flex min-w-0 flex-1 items-baseline justify-between gap-3">
                    <span className="truncate text-xs font-medium text-foreground">{stageLabel(step)}</span>
                    <span className="shrink-0 font-mono text-[10px] text-muted-foreground">{duration < 1000 ? `${duration}ms` : `${(duration / 1000).toFixed(1)}s`}</span>
                  </span>
                </li>
              );
            })}
          </ol>
          {reasoning ? (
            <div className="border-t border-border/70 px-4 py-3">
              <p className="mb-2 text-xs font-medium text-muted-foreground">{t("ai.trace.thinking")}</p>
              <pre
                ref={reasoningRef}
                className="max-h-64 overflow-auto whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-muted-foreground"
              >{reasoning}</pre>
            </div>
          ) : null}
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}
