"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";
import { COMMAND_TYPES } from "./types";

interface BulkCommandModalProps {
  open: boolean;
  selectedCount: number;
  cmdType: string; setCmdType: (v: string) => void;
  cmdText: string; setCmdText: (v: string) => void;
  onSubmit: () => void;
  onClose: () => void;
}

export function BulkCommandModal({
  open, selectedCount,
  cmdType, setCmdType,
  cmdText, setCmdText,
  onSubmit, onClose,
}: BulkCommandModalProps) {
  const { t } = useI18n();
  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>{t("agents.execute_command")}</DialogTitle>
        </DialogHeader>
        <p className="text-xs text-muted-foreground">
          {t("agents.send_command_to").replace("{n}", String(selectedCount)).replace("{s}", selectedCount !== 1 ? "s" : "")}
        </p>
        <div className="space-y-3">
          <div>
            <span className="block text-xs font-medium text-muted-foreground mb-1">{t("agents.command_type")}</span>
            <Select value={cmdType} onValueChange={(v) => { if (v !== null) setCmdType(v); }}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {COMMAND_TYPES.map((ct) => (
                  <SelectItem key={ct.value} value={ct.value}>{ct.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {cmdType !== "screenshot" && cmdType !== "kill" && cmdType !== "uninstall" && (
            <div>
              <span className="block text-xs font-medium text-muted-foreground mb-1">
                {cmdType === "sleep" ? t("agents.interval_seconds") : t("agents.command")}
              </span>
              <Input name="input-7"
                type="text"
                value={cmdText}
                onChange={(e) => setCmdText(e.target.value)}
                placeholder={cmdType === "shell" ? "whoami" : cmdType === "ps" ? "" : cmdType === "ls" ? "C:\\" : "30"}
                className="h-11"
                autoFocus
                onKeyDown={(e) => { if (e.key === "Enter") onSubmit(); }}
              />
            </div>
          )}
          {cmdType === "screenshot" && (
            <p className="text-xs text-muted-foreground/70">{t("agents.no_params_needed")}</p>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={onSubmit} disabled={selectedCount === 0}>{t("agents.execute_btn")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
