"use client";

import { useState, useCallback, useRef } from "react";

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
  const resolverRef = useRef<((value: boolean) => void) | null>(null);

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
        resolverRef.current = resolve;
      });
    },
    []
  );

  const onConfirm = useCallback(() => {
    setOpen(false);
    resolverRef.current?.(true);
    resolverRef.current = null;
  }, []);

  const close = useCallback(() => {
    setOpen(false);
    resolverRef.current?.(false);
    resolverRef.current = null;
  }, []);

  return { open, title, description, confirmLabel, variant, onConfirm, ask, close };
}
