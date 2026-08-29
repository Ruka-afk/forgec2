"use client";

import { useCallback, useRef, useState } from "react";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useI18n } from "@/lib/i18n";

interface ConfirmOptions {
  title?: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  requireText?: string;
}

interface ConfirmState {
  opts: ConfirmOptions;
  resolve: (value: boolean) => void;
}

/**
 * Promise-based confirmation dialog. Returns a boolean promise that resolves
 * with the user's choice, and renders the modal via `confirm.modal`.
 *
 *   const confirm = useConfirm();
 *   const ok = await confirm({ message: "Delete?", danger: true });
 *   if (ok) { ... }
 *   ...
 *   {confirm.modal}
 */
export function useConfirm() {
  const { t } = useI18n();
  const [state, setState] = useState<ConfirmState | null>(null);
  const stateRef = useRef<ConfirmState | null>(null);

  // G3 fix: when a second confirm() is called while one is already open,
  // resolve the pending promise as false before overwriting, preventing hangs.
  const confirm = useCallback((opts: ConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
      if (stateRef.current) stateRef.current.resolve(false);
      stateRef.current = { opts, resolve };
      setState(stateRef.current);
    });
  }, []);

  const close = useCallback((value: boolean) => {
    stateRef.current?.resolve(value);
    stateRef.current = null;
    setState(null);
  }, []);

  const modal = state ? (
    <ConfirmModal
      open
      title={state.opts.title || t("common.confirm")}
      message={state.opts.message}
      confirmText={state.opts.confirmText}
      cancelText={state.opts.cancelText}
      danger={state.opts.danger}
      requireText={state.opts.requireText}
      onConfirm={() => close(true)}
      onCancel={() => close(false)}
    />
  ) : null;

  return { confirm, modal };
}
