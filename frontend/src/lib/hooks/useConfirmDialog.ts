"use client";

import { useState, useCallback } from "react";

interface UseConfirmDialogResult {
  open: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  variant: "default" | "destructive";
  onConfirm: () => void;
  ask: (opts: {
    title: string;
    description: string;
    confirmLabel?: string;
    variant?: "default" | "destructive";
  }) => Promise<boolean>;
  close: () => void;
}

export function useConfirmDialog(): UseConfirmDialogResult {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [confirmLabel, setConfirmLabel] = useState("Confirm");
  const [variant, setVariant] = useState<"default" | "destructive">("default");
  const [resolver, setResolver] = useState<((value: boolean) => void) | null>(null);

  const ask = useCallback(
    (opts: {
      title: string;
      description: string;
      confirmLabel?: string;
      variant?: "default" | "destructive";
    }): Promise<boolean> => {
      setTitle(opts.title);
      setDescription(opts.description);
      setConfirmLabel(opts.confirmLabel ?? "Confirm");
      setVariant(opts.variant ?? "default");
      setOpen(true);
      return new Promise<boolean>((resolve) => {
        setResolver(() => resolve);
      });
    },
    []
  );

  const onConfirm = useCallback(() => {
    setOpen(false);
    resolver?.(true);
    setResolver(null);
  }, [resolver]);

  const close = useCallback(() => {
    setOpen(false);
    resolver?.(false);
    setResolver(null);
  }, [resolver]);

  return { open, title, description, confirmLabel, variant, onConfirm, ask, close };
}
