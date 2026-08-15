"use client";

import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Send, Square } from "lucide-react";

interface AIComposerProps {
  input: string;
  loading: boolean;
  messageCount: number;
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
  textareaRef,
  onChange,
  onKeyDown,
  onSend,
  onStop,
}: AIComposerProps) {
  const { t } = useI18n();
  return (
    <Card className="shrink-0 p-2 sm:p-3">
      <div className="flex items-end gap-2">
        <Textarea
          ref={textareaRef}
          rows={1}
          placeholder={t("ai.input_placeholder")}
          aria-label={t("ai.input_placeholder")}
          className="flex-1 resize-none border-0 focus:ring-0 text-sm py-2.5 px-2 max-h-32 bg-transparent text-foreground placeholder:text-muted-foreground outline-none"
          value={input}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={onKeyDown}
        />
        {loading ? (
          <Button variant="destructive" size="icon" onClick={onStop} className="shrink-0" aria-label={t("ai.stop_generation")}>
            <Square className="w-4 h-4" />
          </Button>
        ) : (
          <Button size="icon" onClick={onSend} className="shrink-0" aria-label={t("ai.send_message")}>
            <Send className="w-4 h-4" />
          </Button>
        )}
      </div>
      <div className="flex justify-between items-center mt-1.5 px-1">
        <span className="text-(--fs-micro-sm) text-muted-foreground">
          {messageCount}/40
          {input.trim() && <> &middot; ~{Math.ceil(input.trim().length / 4)} {t("ai.tokens_est")}</>}
        </span>
        <span className="text-(--fs-micro-sm) text-muted-foreground hidden sm:inline">{t("ai.input_hint")}</span>
      </div>
    </Card>
  );
}
