"use client";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";

interface BatchSleepModalProps {
  agentCount: number;
  interval: string; onIntervalChange: (v: string) => void;
  jitter: string; setJitter: (v: string) => void;
  onSubmit: () => void;
  onClose: () => void;
}

export function BatchSleepModal({ agentCount, interval, onIntervalChange, jitter, setJitter, onSubmit, onClose }: BatchSleepModalProps) {
  const { t } = useI18n();
  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent showCloseButton={false} className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{t("agents.set_sleep_params")}</DialogTitle>
        </DialogHeader>
        <p className="text-xs text-muted-foreground">
          {t("agents.update_sleep_for").replace("{n}", String(agentCount)).replace("{s}", agentCount !== 1 ? "s" : "")}
        </p>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <span className="block text-xs font-medium text-muted-foreground mb-1">{t("agents.interval_s")}</span>
            <Input type="number" value={interval} onChange={(e) => onIntervalChange(e.target.value)} min={5} max={3600}
              className="h-10" />
          </div>
          <div>
            <span className="block text-xs font-medium text-muted-foreground mb-1">{t("agents.jitter_pct")}</span>
            <Input type="number" value={jitter} onChange={(e) => setJitter(e.target.value)} min={0} max={100}
              className="h-10" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={onSubmit}>{t("common.confirm")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
