import { useEffect, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { useMutation } from "@/lib/hooks/useMutation";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface BreakerConfig {
  failure_threshold: number;
  cooldown_seconds: number;
  half_open_max_reqs: number;
  health_check_seconds: number;
}

const EMPTY_CONFIG: BreakerConfig = {
  failure_threshold: 3,
  cooldown_seconds: 300,
  half_open_max_reqs: 3,
  health_check_seconds: 60,
};

interface ListenerBreakerConfigDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ListenerBreakerConfigDialog({ open, onOpenChange }: ListenerBreakerConfigDialogProps) {
  const { t } = useI18n();
  const [form, setForm] = useState<BreakerConfig>(EMPTY_CONFIG);
  // The form is prefilled from the server; until that read succeeds the values
  // on screen are placeholders, so saving them would reset the real thresholds.
  const [configLoaded, setConfigLoaded] = useState(false);
  const [configError, setConfigError] = useState<string | null>(null);
  const [reloadNonce, setReloadNonce] = useState(0);

  const { mutate: save, isPending: saving } = useMutation({
    fn: async () => {
      if (!configLoaded) throw new Error("breaker config unread");
      await api.postJson(paths.circuitBreaker.config, form);
    },
    onSuccess: () => {
      toast.success(t("cb.config_saved"));
      onOpenChange(false);
    },
    onError: () => toast.error(t("cb.config_save_failed")),
  });

  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    setConfigLoaded(false);
    setConfigError(null);
    api
      .get<Partial<BreakerConfig>>(paths.circuitBreaker.config, { signal: controller.signal })
      .then((data) => {
        if (controller.signal.aborted) return;
        setForm({
          failure_threshold: data.failure_threshold ?? EMPTY_CONFIG.failure_threshold,
          cooldown_seconds: data.cooldown_seconds ?? EMPTY_CONFIG.cooldown_seconds,
          half_open_max_reqs: data.half_open_max_reqs ?? EMPTY_CONFIG.half_open_max_reqs,
          health_check_seconds: data.health_check_seconds ?? EMPTY_CONFIG.health_check_seconds,
        });
        setConfigLoaded(true);
      })
      .catch((e: unknown) => {
        if (controller.signal.aborted) return;
        setConfigError(e instanceof Error ? e.message : t("cb.load_failed"));
        toast.error(t("cb.load_failed"));
      });
    return () => controller.abort();
  }, [open, t, reloadNonce]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("cb.config_title")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          {configError && (
            <div role="alert" className="rounded-lg border border-destructive/25 bg-destructive/10 p-3 text-xs text-destructive">
              <p className="font-medium">{t("cb.config_unread")}</p>
              <p className="mt-1 text-muted-foreground">{configError}</p>
              <Button type="button" size="sm" variant="outline" className="mt-2" onClick={() => setReloadNonce((n) => n + 1)}>
                {t("common.try_again")}
              </Button>
            </div>
          )}
          <fieldset disabled={!configLoaded} className="space-y-4">
          <div>
            <Label>{t("cb.failure_threshold")}</Label>
            <Input
              type="number"
              className="mt-1"
              min={1}
              value={form.failure_threshold}
              onChange={(e) => setForm({ ...form, failure_threshold: parseInt(e.target.value, 10) || 3 })}
            />
            <p className="mt-1 text-(--fs-micro-sm) text-muted-foreground">{t("cb.failure_threshold_desc")}</p>
          </div>
          <div>
            <Label>{t("cb.cooldown")}</Label>
            <Input
              type="number"
              className="mt-1"
              min={10}
              value={form.cooldown_seconds}
              onChange={(e) => setForm({ ...form, cooldown_seconds: parseInt(e.target.value, 10) || 300 })}
            />
            <p className="mt-1 text-(--fs-micro-sm) text-muted-foreground">{t("cb.cooldown_desc")}</p>
          </div>
          <div>
            <Label>{t("cb.half_open_max")}</Label>
            <Input
              type="number"
              className="mt-1"
              min={1}
              value={form.half_open_max_reqs}
              onChange={(e) => setForm({ ...form, half_open_max_reqs: parseInt(e.target.value, 10) || 3 })}
            />
            <p className="mt-1 text-(--fs-micro-sm) text-muted-foreground">{t("cb.half_open_max_desc")}</p>
          </div>
          <div>
            <Label>{t("cb.health_check_interval")}</Label>
            <Input
              type="number"
              className="mt-1"
              min={5}
              value={form.health_check_seconds}
              onChange={(e) => setForm({ ...form, health_check_seconds: parseInt(e.target.value, 10) || 60 })}
            />
            <p className="mt-1 text-(--fs-micro-sm) text-muted-foreground">{t("cb.health_check_desc")}</p>
          </div>
          </fieldset>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>{t("common.cancel")}</Button>
          <Button onClick={() => void save()} disabled={saving || !configLoaded}>{t("cb.save_config")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
