"use client";

import { memo } from "react";
import { TableCell, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { CopyButton } from "@/components/ui/copy-button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  CircleCheck,
  CircleQuestionMark,
  Database,
  Eye,
  EyeOff,
  PenLine,
  Pencil,
  Shield,
  Trash2,
  WandSparkles,
} from "lucide-react";
import { TYPE_BADGE_VARIANT, type VaultEntry } from "./types";

interface CredentialRowProps {
  entry: VaultEntry;
  isSelected: boolean;
  showPassword: boolean;
  onToggleSelect: (id: string) => void;
  onToggleConfirm: (entry: VaultEntry) => void;
  onEdit: (entry: VaultEntry) => void;
  onDelete: (id: string) => void;
  togglePasswordVisibility: (id: string) => void;
  t: (key: string) => string;
}

function CredentialRowInner({
  entry,
  isSelected,
  showPassword,
  onToggleSelect,
  onToggleConfirm,
  onEdit,
  onDelete,
  togglePasswordVisibility,
  t,
}: CredentialRowProps) {
  return (
    <TableRow className="hover:bg-muted/50 transition-colors">
      <TableCell className="py-3 px-2">
        <Checkbox checked={isSelected} onCheckedChange={() => onToggleSelect(entry.id)} aria-label={t("cred.select_entry")} />
      </TableCell>
      <TableCell className="py-3 px-4">
        <Badge variant={TYPE_BADGE_VARIANT[entry.type] || "outline"}>
          {entry.type || "unknown"}
        </Badge>
      </TableCell>
      <TableCell className="py-3 px-4 font-medium text-foreground">{entry.username}</TableCell>
      <TableCell className="py-3 px-4 font-mono text-xs">
        {entry.password ? (
          <div className="flex items-center gap-1">
            <span className="text-muted-foreground">
              {showPassword ? entry.password : "????????"}
            </span>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => togglePasswordVisibility(entry.id)}
              aria-label={showPassword ? t("cred.hide_password") : t("cred.show_password")}
            >
              {showPassword ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
            </Button>
            <CopyButton
              text={entry.password}
              label={t("cred.copy_password")}
              title={t("cred.copy_password")}
              size="icon-xs"
            />
          </div>
        ) : (
          <span className="text-muted-foreground">-</span>
        )}
      </TableCell>
      <TableCell className="max-sm:hidden py-3 px-4 text-muted-foreground">{entry.domain || "-"}</TableCell>
      <TableCell className="max-sm:hidden py-3 px-4 text-xs text-muted-foreground">
        {entry.source === "mimikatz" ? <WandSparkles className="w-3 h-3 text-amber-600 dark:text-amber-400 mr-1" /> : entry.source === "sam" ? <Database className="w-3 h-3 text-blue-500 mr-1" /> : entry.source === "kerberoast" ? <Shield className="w-3 h-3 text-orange-500 mr-1" /> : <PenLine className="w-3 h-3 text-muted-foreground mr-1" />}
        {entry.source || "manual"}
      </TableCell>
      <TableCell className="max-sm:hidden py-3 px-4">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => onToggleConfirm(entry)}
          className={entry.confirmed ? "text-primary hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-900/30" : "text-muted-foreground hover:bg-muted"}
        >
          {entry.confirmed ? <CircleCheck className="w-4 h-4" /> : <CircleQuestionMark className="w-4 h-4" />}
          {entry.confirmed ? t("cred.confirmed") : t("cred.unconfirmed")}
        </Button>
      </TableCell>
      <TableCell className="max-sm:hidden py-3 px-4">
        {entry.tags ? (
          <div className="flex flex-wrap gap-1">
            {entry.tags.split(",").map(tag => (
              <Badge key={tag} variant="outline">{tag.trim()}</Badge>
            ))}
          </div>
        ) : (
          <span className="text-muted-foreground text-xs">-</span>
        )}
      </TableCell>
      <TableCell className="py-3 px-4 text-center whitespace-nowrap">
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={() => onEdit(entry)} aria-label={t("cred.edit_title")} />}>
            <Pencil className="w-4 h-4" />
          </TooltipTrigger>
          <TooltipContent>{t("common.edit")}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={() => onDelete(entry.id)} className="text-destructive hover:text-destructive/80 hover:bg-destructive/5" aria-label={t("cred.delete_title")} />}>
            <Trash2 className="w-4 h-4" />
          </TooltipTrigger>
          <TooltipContent>{t("common.delete")}</TooltipContent>
        </Tooltip>
      </TableCell>
    </TableRow>
  );
}

export const CredentialRow = memo(CredentialRowInner);
