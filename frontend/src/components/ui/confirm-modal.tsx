"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";

export function ConfirmModal({ open, title, message, confirmText, cancelText, danger, requireText, onConfirm, onCancel }: {
  open: boolean;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  requireText?: string;
  onConfirm: () => void | Promise<void>;
  onCancel: () => void;
}) {
  const { t } = useI18n();
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState(false);
  const resolvedConfirm = confirmText || t("common.confirm");
  const resolvedCancel = cancelText || t("common.cancel");
  const matched = !requireText || typed.trim() === requireText;

  useEffect(() => {
    if (open) {
      setTyped("");
      setBusy(false);
    }
  }, [open]);

  const handleConfirm = () => {
    if (busy) return;
    setBusy(true);
    try {
      const result = onConfirm();
      if (result instanceof Promise) {
        result.catch(() => {}).finally(() => setBusy(false));
      } else {
        setBusy(false);
      }
    } catch {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-muted-foreground">{message}</p>
        {requireText && (
          <div>
            <label htmlFor="confirm-modal-typed" className="block text-xs text-muted-foreground mb-1.5">
              {t("common.type_to_confirm").replace("{text}", requireText)}
            </label>
            <Input
              id="confirm-modal-typed"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              placeholder={requireText}
              className="font-mono"
            />
          </div>
        )}
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel} disabled={busy}>{resolvedCancel}</Button>
          <Button variant={danger ? "destructive" : "default"} disabled={!matched || busy} onClick={handleConfirm}>{resolvedConfirm}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
