import { useState } from "react";
import { useConfirm } from "@/lib/hooks/useConfirm";

export function useInfrastructureDialogs() {
  const [showGenModal, setShowGenModal] = useState(false);
  const { confirm, modal } = useConfirm();
  const [editingRd, setEditingRd] = useState<number | null>(null);

  return { showGenModal, setShowGenModal, confirm, modal, editingRd, setEditingRd };
}
