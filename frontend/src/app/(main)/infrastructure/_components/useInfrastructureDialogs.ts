import { useState } from "react";

export interface ConfirmDialog {
  msg: string;
  cb: () => void;
}

export function useInfrastructureDialogs() {
  const [showGenModal, setShowGenModal] = useState(false);
  const [cfm, setCfm] = useState<ConfirmDialog | null>(null);
  const [editingRd, setEditingRd] = useState<number | null>(null);

  return { showGenModal, setShowGenModal, cfm, setCfm, editingRd, setEditingRd };
}
