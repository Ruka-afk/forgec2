"use client";

import { memo } from "react";
import { Badge } from "@/components/ui/badge";

interface OperatorBadgeProps {
  username: string;
  isCurrentUser?: boolean;
  size?: "sm" | "md";
  showDot?: boolean;
}

function OperatorBadgeInner({ username, isCurrentUser, size = "sm", showDot = true }: OperatorBadgeProps) {
  return (
    <Badge
      variant={isCurrentUser ? "default" : "secondary"}
      className={`inline-flex items-center gap-1.5 px-1.5 py-0.5 ${size === "sm" ? "text-(--font-size-xs-sm)" : "text-xs"} ${
        isCurrentUser
          ? "bg-primary/10 text-primary"
          : "bg-primary/10 text-primary"
      }`}
    >
      {showDot && (
        <span className={`w-1.5 h-1.5 rounded-full ${
          isCurrentUser ? "bg-primary" : "bg-emerald-500"
        } ${!isCurrentUser ? "animate-pulse-glow" : ""}`} />
      )}
      {username}{isCurrentUser ? " (you)" : ""}
    </Badge>
  );
}

const OperatorBadge = memo(OperatorBadgeInner);
export default OperatorBadge;
