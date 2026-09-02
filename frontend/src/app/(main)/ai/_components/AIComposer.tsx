"use client";

import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Send, Square } from "lucide-react";

interface AIComposerProps {
  input: string;
  loading: boolean;
  messageCount: number;
  disabled?: boolean;
  usage?: { prompt: number; completion: number };
  maxLength?: number;
  textareaRef: React.RefObject<HTMLTextAreaElement | null>;
  onChange: (value: string) => void;
  onKeyDown: (e: React.KeyboardEvent) => void;
  onSend: () => void;
  onStop: () => void;
}

export function AIComposer({
  input,
  loading,
  messageCount,
  disabled,
  usage,
  maxLength,
  textareaRef,
  onChange,
  onKeyDown,
  onSend,
  onStop,
}: AIComposerProps) {
  const { t } = useI18n();
  return (
    <div className="overflow-hidden rounded-2xl border border-border/90 bg-card shadow-lg shadow-foreground/5 transition-[border-color,box-shadow] duration-150 focus-within:border-primary/35 focus-within:shadow-xl focus-within:shadow-primary/8">
      <div className="flex items-end gap-2 p-2 sm:p-2.5">
        <Textarea
          ref={textareaRef}
          rows={1}
          placeholder={disabled ? t("ai.configure_first") : t("ai.input_placeholder")}
          aria-label={t("ai.input_placeholder")}
          className="min-h-11 max-h-32 flex-1 resize-none border-0 bg-transparent px-2 py-2.5 text-sm leading-6 text-foreground outline-none placeholder:text-muted-foreground focus:ring-0"
          value={input}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={onKeyDown}
          disabled={disabled}
          aria-busy={loading}
          maxLength={maxLength}
        />
        {loading ? (
          <Button variant="destructive" size="icon" onClick={onStop} className="mb-0.5 shrink-0 rounded-xl" aria-label={t("ai.stop_generation")}>
            <Square className="size-4" />
          </Button>
        ) : (
          <Button size="icon" onClick={onSend} className="mb-0.5 shrink-0 rounded-xl" disabled={disabled || !input.trim()} aria-label={t("ai.send_message")}>
            <Send className="size-4" />
          </Button>
        )}
      </div>
      <div className="flex items-center justify-between gap-3 border-t border-border/60 bg-muted/30 px-3 py-2">
        <span className="text-(--fs-micro-sm) text-muted-foreground">
          {messageCount}
          {maxLength && <> &middot; {input.length}/{maxLength}</>}
          {input.trim() && <> &middot; ~{Math.ceil(input.trim().length / 4)} {t("ai.tokens_est")}</>}
        </span>
        <span className="flex items-center gap-3 text-(--fs-micro-sm) text-muted-foreground">
          {usage && (usage.prompt > 0 || usage.completion > 0) && (
            <span title={t("ai.tokens_used")} className="font-mono">
              ↑{usage.prompt} ↓{usage.completion}
            </span>
          )}
          <span className="hidden sm:inline">
            {loading ? t("ai.input_during_generation") : t("ai.input_hint")}
          </span>
        </span>
      </div>
    </div>
  );
}
