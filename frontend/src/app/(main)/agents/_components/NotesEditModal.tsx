"use client";

import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";

interface NotesEditModalProps {
  notesText: string; setNotesText: (v: string) => void;
  onSubmit: () => void;
  onClose: () => void;
}

export function NotesEditModal({
  notesText, setNotesText, onSubmit, onClose,
}: NotesEditModalProps) {
  const { t } = useI18n();
  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>{t("agents.edit_notes")}</DialogTitle>
        </DialogHeader>
        <Textarea
          value={notesText}
          onChange={(e) => setNotesText(e.target.value)}
          placeholder={t("agents.add_notes_placeholder")}
          rows={4}
          autoFocus
        />
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={onSubmit}>{t("common.save")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
