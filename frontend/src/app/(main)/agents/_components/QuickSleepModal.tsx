"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";
import { Monitor } from "lucide-react";

interface QuickSleepModalProps {
  agent: { id: string; hostname: string; interval: number; jitter: number };
  sleepInterval: string; setSleepInterval: (v: string) => void;
  sleepJitter: string; setSleepJitter: (v: string) => void;
  onSubmit: () => void;
  onClose: () => void;
}

export function QuickSleepModal({
  agent, sleepInterval, setSleepInterval, sleepJitter, setSleepJitter, onSubmit, onClose,
}: QuickSleepModalProps) {
  const { t } = useI18n();
  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent showCloseButton={false} className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{t("agents.adjust_sleep")}</DialogTitle>
        </DialogHeader>
        <p className="text-xs text-muted-foreground">
          <Monitor className="size-4" />{agent.hostname}
        </p>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <span className="block text-xs font-medium text-muted-foreground mb-1">{t("agents.interval_s")}</span>
            <Input type="number" value={sleepInterval} onChange={(e) => setSleepInterval(e.target.value)} min={5} max={3600}
              className="h-10" />
          </div>
          <div>
            <span className="block text-xs font-medium text-muted-foreground mb-1">{t("agents.jitter_pct")}</span>
            <Input type="number" value={sleepJitter} onChange={(e) => setSleepJitter(e.target.value)} min={0} max={100}
              className="h-10" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={onSubmit}>{t("agents.apply")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
