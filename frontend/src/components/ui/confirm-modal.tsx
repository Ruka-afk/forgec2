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
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const { t } = useI18n();
  const [typed, setTyped] = useState("");
  const resolvedConfirm = confirmText || t("common.confirm");
  const resolvedCancel = cancelText || t("common.cancel");
  const matched = !requireText || typed.trim() === requireText;

  useEffect(() => {
    if (open) setTyped("");
  }, [open]);

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
          <Button variant="ghost" onClick={onCancel}>{resolvedCancel}</Button>
          <Button variant={danger ? "destructive" : "default"} disabled={!matched} onClick={onConfirm}>{resolvedConfirm}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
