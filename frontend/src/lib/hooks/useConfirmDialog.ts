"use client";

import { useState, useCallback } from "react";

interface ConfirmDialogState {
  message: string;
  onConfirm: () => void;
}

export function useConfirmDialog() {
  const [dialog, setDialog] = useState<ConfirmDialogState | null>(null);

  const confirm = useCallback((message: string, onConfirm: () => void) => {
    setDialog({ message, onConfirm });
  }, []);

  const cancel = useCallback(() => setDialog(null), []);

  return { dialog, confirm, cancel };
}
