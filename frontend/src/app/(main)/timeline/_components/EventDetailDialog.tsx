"use client";

import Link from "next/link";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { ExternalLink } from "lucide-react";
import { EVENT_COLORS } from "../page";
import type { TimelineEvent } from "../page";
import { safeHref } from "@/lib/safeUrl";

interface EventDetailDialogProps {
  event: TimelineEvent | null;
  onClose: () => void;
}

function colorFor(type: string) {
  return EVENT_COLORS[type] || { dot: "bg-muted-foreground", bg: "bg-muted", text: "text-muted-foreground", icon: null };
}

function variantFor(type: string): "success" | "destructive" | "warning" | "outline" {
  if (type === "agent_online") return "success";
  if (type === "alert") return "destructive";
  if (type === "credential") return "warning";
  return "outline";
}

const getEventTime = (e: TimelineEvent) => e.timestamp || "";
const getEventType = (e: TimelineEvent) => (e.type || "").toLowerCase();
const getEventTitle = (e: TimelineEvent) => e.title || "";
const getEventDesc = (e: TimelineEvent) => e.description || "";
const getEventUser = (e: TimelineEvent) => e.username || "";
const getEventAgent = (e: TimelineEvent) => e.agent_id || "";
const getEventUrl = (e: TimelineEvent) => e.url || "";

export default function EventDetailDialog({ event, onClose }: EventDetailDialogProps) {
  const { t } = useI18n();

  return (
    <Dialog open={!!event} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-lg max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("timeline.event_details")}</DialogTitle>
        </DialogHeader>
        {event && (<div className="space-y-4">
          <div>
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.time")}</span>
            <p className="text-sm font-mono mt-0.5">{getEventTime(event)}</p>
          </div>
          <div>
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.type")}</span>
            <p className="text-sm mt-0.5">
              <Badge variant={variantFor(getEventType(event))}>
                <span className={`w-1.5 h-1.5 rounded-full ${colorFor(getEventType(event)).dot}`}></span>
                {getEventType(event)}
              </Badge>
            </p>
          </div>
          <div>
            <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.title")}</span>
            <p className="text-sm mt-0.5">{getEventTitle(event)}</p>
          </div>
          {getEventDesc(event) && (
            <div>
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.description")}</span>
              <p className="text-sm text-muted-foreground mt-0.5">{getEventDesc(event)}</p>
            </div>
          )}
          {getEventUser(event) && (
            <div>
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.user")}</span>
              <p className="text-sm mt-0.5">{getEventUser(event)}</p>
            </div>
          )}
          {getEventAgent(event) && (
            <div>
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("timeline.col_agent_id")}</span>
              <p className="text-sm font-mono mt-0.5">{getEventAgent(event)}</p>
            </div>
          )}
          {getEventUrl(event) && safeHref(getEventUrl(event)) && (
            <div className="pt-2">
              <Link href={safeHref(getEventUrl(event))!}>
                <Button className="gap-2">
                  <ExternalLink className="w-4 h-4" />
                  <span>{t("timeline.view_related")}</span>
                </Button>
              </Link>
            </div>
          )}
        </div>)}
      </DialogContent>
    </Dialog>
  );
}
