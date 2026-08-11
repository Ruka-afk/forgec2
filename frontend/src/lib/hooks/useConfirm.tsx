"use client";

import { useCallback, useRef, useState } from "react";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { useI18n } from "@/lib/i18n";

export interface ConfirmOptions {
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

  const confirm = useCallback((opts: ConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
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
