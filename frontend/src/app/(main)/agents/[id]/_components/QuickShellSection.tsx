"use client";

import { memo } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { SectionCard } from "@/components/ui/section-card";
import { timeAgo } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { Send, Terminal } from "lucide-react";

interface QuickShellEntry {
  command: string;
  shell: string;
  result: string;
  timestamp: string;
}

interface QuickShellSectionProps {
  expanded: boolean;
  onExpandedChange: (v: boolean) => void;
  shellInterpreter: string;
  onShellChange: (v: string) => void;
  command: string;
  onCommandChange: (v: string) => void;
  history: QuickShellEntry[];
  sending: boolean;
  onSend: () => void;
  os?: string;
}

export default memo(function QuickShellSection({
  expanded,
  onExpandedChange,
  shellInterpreter,
  onShellChange,
  command,
  onCommandChange,
  history,
  sending,
  onSend,
  os,
}: QuickShellSectionProps) {
  const { t } = useI18n();
  const isWindows = !os || /^win/i.test(os);

  return (
    <SectionCard
      className="mb-4"
      title={t("agents.detail_quick_shell")}
      icon={<Terminal className="w-3.5 h-3.5" />}
      collapsible
      defaultOpen={expanded}
      onOpenChange={onExpandedChange}
    >
      <div className="p-4">
        <div className="flex items-center gap-2 mb-3">
          <Select value={shellInterpreter} onValueChange={(v) => { if (v !== null) onShellChange(v); }}>
            <SelectTrigger className="h-8 w-[160px] text-xs" aria-label={t("agents.detail_quick_shell")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {isWindows ? (
                <>
                  <SelectItem value="cmd.exe">cmd.exe</SelectItem>
                  <SelectItem value="powershell.exe">powershell.exe</SelectItem>
                </>
              ) : (
                <>
                  <SelectItem value="/bin/sh">/bin/sh</SelectItem>
                  <SelectItem value="/bin/bash">/bin/bash</SelectItem>
                </>
              )}
            </SelectContent>
          </Select>
          <Input
            type="text"
            value={command}
            onChange={(e) => onCommandChange(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") onSend(); }}
            placeholder={t("agents.detail_enter_command")}
            aria-label={t("agents.detail_enter_command")}
            className="flex-1 h-8 font-mono text-xs"
          />
          <Button size="sm" onClick={onSend} disabled={sending || !command.trim()} className="h-8 px-4 text-xs gap-1.5 shrink-0">
            {sending ? <Spinner size="xs" color="white" /> : <Send className="w-4 h-4" />} {t("agents.detail_send")}
          </Button>
        </div>
        {history.length > 0 && (
          <div className="space-y-2 max-h-60 overflow-y-auto">
            {history.map((entry, idx) => (
              <div key={entry.timestamp + idx} className="p-2.5 rounded-lg bg-muted/50 border border-border">
                <div className="flex items-center gap-2 mb-1">
                  <Badge variant="secondary" className="text-(--fs-micro-sm) font-mono">{entry.shell}</Badge>
                  <span className="text-xs font-mono text-foreground">{entry.command}</span>
                  <span className="text-(--fs-micro-sm) text-muted-foreground/70 ml-auto">{timeAgo(entry.timestamp, t)}</span>
                </div>
                <pre className="font-mono text-(--fs-micro-sm) text-muted-foreground whitespace-pre-wrap break-all max-h-20 overflow-y-auto">{entry.result}</pre>
              </div>
            ))}
          </div>
        )}
      </div>
    </SectionCard>
  );
});