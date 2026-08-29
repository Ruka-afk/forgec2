"use client";

import { useI18n } from "@/lib/i18n";
import { formatTime } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { StatusIndicator } from "@/components/ui/status-indicator";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { RotateCw } from "lucide-react";
import {
  healthIndicatorStatus,
  translateHealthStatus,
  type ListenerHealth,
} from "./listener-health";

interface ListenerHealthCellProps {
  health?: ListenerHealth;
  onReset?: () => void;
}

export function ListenerHealthCell({ health, onReset }: ListenerHealthCellProps) {
  const { t } = useI18n();
  if (!health) {
    return (
      <span className="text-xs text-muted-foreground">{t("listeners.health_unmonitored")}</span>
    );
  }

  const status = healthIndicatorStatus(health.status);
  const fails = health.consecutive_fails ?? 0;
  const reasons = (health.fail_reasons || []).filter(Boolean);
  const label = translateHealthStatus(t, health.status);
  const tip = [
    label,
    fails > 0 ? t("listeners.health_fails", { n: fails }) : null,
    t("listeners.health_last_probe", { time: health.last_probe ? formatTime(health.last_probe) : t("cb.never") }),
    ...reasons.slice(0, 3),
  ].filter(Boolean).join(" · ");

  return (
    <div className="flex items-center gap-1.5">
      <Tooltip>
        <TooltipTrigger
          render={
            <span className="inline-flex min-w-0">
              <StatusIndicator
                status={status}
                variant="dot"
                label={label}
                pulse={status === "burned"}
              />
            </span>
          }
        />
        <TooltipContent className="max-w-xs">{tip}</TooltipContent>
      </Tooltip>
      {fails > 0 && (
        <span className={`font-mono text-(--fs-micro-sm) ${fails >= 3 ? "text-destructive" : "text-warning"}`}>
          {fails}
        </span>
      )}
      {onReset && (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon-xs"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  onReset();
                }}
                aria-label={t("listeners.reset_health")}
                className="text-muted-foreground"
              />
            }
          >
            <RotateCw className="size-3" />
          </TooltipTrigger>
          <TooltipContent>{t("listeners.reset_health")}</TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}
