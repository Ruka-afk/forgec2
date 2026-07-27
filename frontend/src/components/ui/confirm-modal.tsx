"use client";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";

export function ConfirmModal({ open, title, message, confirmText, cancelText, danger, onConfirm, onCancel }: {
  open: boolean;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const { t } = useI18n();
  const resolvedConfirm = confirmText || t("common.confirm");
  const resolvedCancel = cancelText || t("common.cancel");
  return (
    <Dialog open={open} onOpenChange={(v) => !v && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-muted-foreground">{message}</p>
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel}>{resolvedCancel}</Button>
          <Button variant={danger ? "destructive" : "default"} onClick={onConfirm}>{resolvedConfirm}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
