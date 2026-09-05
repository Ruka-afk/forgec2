"use client";

import { memo } from "react";
import { Card } from "@/components/ui/card";
import { StatusDot } from "@/components/ui/status-dot";
import { IconBadge } from "@/components/ui/icon-badge";
import { Keyboard, Mouse, Wifi } from "lucide-react";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface RdpStatusCardsProps {
  t: TKey;
  monitoring: boolean;
}

/** Mouse / keyboard / connection status cards below the stage. */
export default memo(function RdpStatusCards({ t, monitoring }: RdpStatusCardsProps) {
  return (
    <div className="mt-3 grid grid-cols-1 sm:grid-cols-3 gap-3 shrink-0">
      <Card className="px-4 py-3 flex flex-row items-center gap-3">
        <IconBadge icon={Mouse} color="primary" size="md" />
        <div>
          <div className="text-(--fs-xs-sm) text-muted-foreground uppercase tracking-wider font-semibold">
            {t("agents.rdp_mouse")}
          </div>
          <div className="text-sm text-foreground">
            {monitoring ? t("agents.rdp_click_move_active") : t("agents.rdp_inactive")}
          </div>
        </div>
      </Card>
      <Card className="px-4 py-3 flex flex-row items-center gap-3">
        <IconBadge icon={Keyboard} color="primary" size="md" />
        <div>
          <div className="text-(--fs-xs-sm) text-muted-foreground uppercase tracking-wider font-semibold">
            {t("agents.rdp_keyboard")}
          </div>
          <div className="text-sm text-foreground">
            {monitoring ? t("agents.rdp_active") : t("agents.rdp_inactive")}
          </div>
        </div>
      </Card>
      <Card className="px-4 py-3 flex flex-row items-center gap-3">
        <IconBadge icon={Wifi} color="primary" size="md" />
        <div>
          <div className="text-(--fs-xs-sm) text-muted-foreground uppercase tracking-wider font-semibold">
            {t("agents.rdp_connection")}
          </div>
          <div className="text-sm text-foreground">
            <span
               className={`inline-flex items-center gap-1 ${monitoring ? "text-success" : "text-muted-foreground/100"}`}
            >
              <StatusDot tone={monitoring ? "success" : "muted"} size="xs" pulse={monitoring} />
              {monitoring ? t("agents.rdp_session_active") : t("agents.rdp_disconnected")}
            </span>
          </div>
        </div>
      </Card>
    </div>
  );
});
