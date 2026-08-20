"use client";

import { memo } from "react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Spinner } from "@/components/ui/spinner";
import { SectionCard } from "@/components/ui/section-card";
import { useI18n } from "@/lib/i18n";
import { Check, Pencil, Tag, X } from "lucide-react";

interface NotesTagsSectionProps {
  editing: boolean;
  tags: string;
  onTagsChange: (v: string) => void;
  notes: string;
  onNotesChange: (v: string) => void;
  saving: boolean;
  onStartEdit: () => void;
  onCancelEdit: () => void;
  onSave: () => void;
  displayTags: string[];
  note: string;
}

export default memo(function NotesTagsSection({
  editing,
  tags,
  onTagsChange,
  notes,
  onNotesChange,
  saving,
  onStartEdit,
  onCancelEdit,
  onSave,
  displayTags,
  note,
}: NotesTagsSectionProps) {
  const { t } = useI18n();

  return (
    <SectionCard
      className="mb-4"
      title={t("agents.detail_notes_tags")}
      icon={<Tag className="w-3.5 h-3.5" />}
      action={!editing ? (
        <Button variant="ghost" size="sm" onClick={onStartEdit} className="text-xs h-auto p-0 text-primary hover:bg-transparent gap-1.5">
          <Pencil className="w-4 h-4" /> {t("agents.detail_edit")}
        </Button>
      ) : (
        <div className="flex items-center gap-2">
          <Button size="sm" onClick={onSave} disabled={saving} className="text-xs h-7 gap-1.5">
            {saving ? <Spinner size="xs" color="white" /> : <Check className="w-4 h-4" />} {t("agents.detail_save")}
          </Button>
          <Button variant="ghost" size="sm" onClick={onCancelEdit} className="text-xs h-7 text-muted-foreground gap-1.5">
            <X className="w-4 h-4" /> {t("agents.detail_cancel")}
          </Button>
        </div>
      )}
    >
      <div className="p-4">
        {editing ? (
          <div className="space-y-3">
            <div>
              <span className="block text-xs font-medium text-muted-foreground mb-1.5">{t("agents.detail_tags_hint")}</span>
              <Input aria-label={t("agents.detail_tags")} name="agent-tags" type="text" value={tags} onChange={(e) => onTagsChange(e.target.value)} placeholder={t("agents.detail_tags_placeholder")} />
            </div>
            <div>
              <span className="block text-xs font-medium text-muted-foreground mb-1.5">{t("agents.detail_notes")}</span>
              <Textarea aria-label={t("agents.detail_notes")} name="agent-notes" value={notes} onChange={(e) => onNotesChange(e.target.value)} rows={3} placeholder={t("agents.detail_notes_placeholder")} />
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <div>
              <div className="text-xs font-medium text-muted-foreground mb-2">{t("agents.detail_tags")}</div>
              {displayTags.length > 0 ? (
                <div className="flex flex-wrap gap-1.5">
                  {displayTags.map((tag) => (
                    <Link key={tag} href={`/agents?tag=${encodeURIComponent(tag)}`}>
                      <Badge variant="outline" className="cursor-pointer hover:opacity-80 transition-opacity">{tag}</Badge>
                    </Link>
                  ))}
                </div>
              ) : <span className="text-xs text-muted-foreground/70">{t("agents.detail_no_tags")}</span>}
            </div>
            <div>
              <div className="text-xs font-medium text-muted-foreground mb-1">{t("agents.detail_notes")}</div>
              {note ? <p className="text-sm text-foreground whitespace-pre-wrap leading-relaxed">{note}</p> : <span className="text-xs text-muted-foreground/70">{t("agents.detail_no_notes")}</span>}
            </div>
          </div>
        )}
      </div>
    </SectionCard>
  );
});