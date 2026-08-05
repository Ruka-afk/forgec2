"use client";

import { memo } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Check, CheckCircle, CircleAlert, Info, Trash2, XCircle } from "lucide-react";
import { formatTime } from "@/lib/utils";

interface NotificationItem {
  id: number;
  type: string;
  title: string;
  message: string;
  agent_id: string;
  task_id: number;
  severity: string;
  read: boolean;
  created_at: string;
}

const SEVERITY_VARIANT: Record<string, "success" | "destructive" | "warning" | "default"> = {
  success: "success",
  error: "destructive",
  warning: "warning",
  info: "default",
};

const SEVERITY_ICONS: Record<string, React.ReactNode> = {
  success: <CheckCircle className="w-4 h-4" />,
  error: <XCircle className="w-4 h-4" />,
  warning: <CircleAlert className="w-4 h-4" />,
  info: <Info className="w-4 h-4" />,
};

interface NotificationRowProps {
  item: NotificationItem;
  isSelected: boolean;
  onToggleSelect: (id: number) => void;
  onMarkRead: (id: number) => void;
  onDelete: (id: number) => void;
  t: (key: string, params?: Record<string, string | number>) => string;
}

function NotificationRowInner({
  item,
  isSelected,
  onToggleSelect,
  onMarkRead,
  onDelete,
  t,
}: NotificationRowProps) {
  const n = item;
  return (
    <div className={`flex items-start gap-3 px-4 sm:px-6 py-4 transition-colors ${n.read ? "" : "bg-primary/5"}`}>
      <Checkbox
        checked={isSelected}
        onCheckedChange={() => onToggleSelect(n.id)}
        aria-label={`Select notification ${n.title}`}
        className="mt-1 shrink-0"
      />
      <div className={`shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-xs ${SEVERITY_VARIANT[n.severity] === "success" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400" : SEVERITY_VARIANT[n.severity] === "destructive" ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400" : SEVERITY_VARIANT[n.severity] === "warning" ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400" : "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"}`}>
        {SEVERITY_ICONS[n.severity] || <Info className="w-4 h-4" />}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-0.5">
          <span className="font-medium text-sm">{n.title || n.type}</span>
          {!n.read && <span className="w-2 h-2 rounded-full bg-primary shrink-0"></span>}
        </div>
        <p className="text-sm text-muted-foreground truncate">{n.message}</p>
        <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
          <span>{n.created_at ? formatTime(n.created_at) : "-"}</span>
          {n.agent_id && <span>{t("notifications.agent_prefix")} {n.agent_id.substring(0, 8)}</span>}
          <Badge variant={SEVERITY_VARIANT[n.severity] || "default"}>{n.severity}</Badge>
          <span className="font-mono text-(--fs-micro-sm)">{n.type}</span>
        </div>
      </div>
      <div className="flex items-center gap-1 shrink-0">
        {!n.read && (
          <Tooltip>
            <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={() => onMarkRead(n.id)} className="w-8 h-8 p-0" aria-label={t("notifications.mark_read")} />}>
              <Check className="w-4 h-4" />
            </TooltipTrigger>
            <TooltipContent>Mark read</TooltipContent>
          </Tooltip>
        )}
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={() => onDelete(n.id)} className="w-8 h-8 p-0 text-muted-foreground hover:text-destructive" aria-label={t("notifications.delete")} />}>
            <Trash2 className="w-4 h-4" />
          </TooltipTrigger>
          <TooltipContent>Delete</TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}

export const NotificationRow = memo(NotificationRowInner);
