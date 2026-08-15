"use client";

import { memo } from "react";
import { StatusDot } from "@/components/ui/status-dot";

interface ConnectionDotProps {
  connected: boolean;
  reconnectFailed?: boolean;
  size?: "xs" | "sm" | "md" | "lg";
  className?: string;
}

/**
 * Shared connection indicator used in the Sidebar and TopBar so the
 * connected / connecting / failed states stay identical across surfaces.
 */
export const ConnectionDot = memo(function ConnectionDot({
  connected,
  reconnectFailed = false,
  size = "sm",
  className,
}: ConnectionDotProps) {
  const tone = reconnectFailed ? "destructive" : connected ? "success" : "warning";
  const pulse = !reconnectFailed;
  return <StatusDot tone={tone} pulse={pulse} size={size} className={className} />;
});
