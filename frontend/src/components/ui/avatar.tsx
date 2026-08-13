"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

const AVATAR_SIZES = {
  sm: "w-7 h-7 text-xs",
  md: "w-9 h-9 text-sm",
  lg: "w-12 h-12 text-base",
} as const;

const AVATAR_SHAPES = {
  circle: "rounded-full",
  square: "rounded-lg",
  xl: "rounded-xl",
} as const;

type AvatarSize = keyof typeof AVATAR_SIZES;
type AvatarShape = keyof typeof AVATAR_SHAPES;

function getInitial(name: string): string {
  return (name || "?").charAt(0).toUpperCase();
}

function hashColor(name: string): { bg: string; fg: string } {
  const palette = [
    { bg: "bg-avatar-1", fg: "text-avatar-1-fg" },
    { bg: "bg-avatar-2", fg: "text-avatar-2-fg" },
    { bg: "bg-avatar-3", fg: "text-avatar-3-fg" },
    { bg: "bg-avatar-4", fg: "text-avatar-4-fg" },
    { bg: "bg-avatar-5", fg: "text-avatar-5-fg" },
    { bg: "bg-avatar-6", fg: "text-avatar-6-fg" },
    { bg: "bg-avatar-7", fg: "text-avatar-7-fg" },
    { bg: "bg-avatar-8", fg: "text-avatar-8-fg" },
  ];
  let hash = 0;
  for (let i = 0; i < (name || "").length; i++) {
    hash = (hash << 5) - hash + (name || "").charCodeAt(i);
    hash |= 0;
  }
  return palette[Math.abs(hash) % palette.length];
}

interface AvatarRootProps extends React.ComponentProps<"span"> {
  size?: AvatarSize;
  shape?: AvatarShape;
}

const AvatarRoot = React.forwardRef<HTMLSpanElement, AvatarRootProps>(
  ({ size = "md", shape = "square", className, ...props }, ref) => (
    <span
      ref={ref}
      role="img"
      aria-label={props["aria-label"] || props.title || undefined}
      className={cn(
        "relative inline-flex shrink-0 items-center justify-center font-bold overflow-hidden",
        AVATAR_SIZES[size],
        AVATAR_SHAPES[shape],
        className
      )}
      {...props}
    />
  )
);
AvatarRoot.displayName = "AvatarRoot";

interface AvatarFallbackProps extends React.ComponentProps<"span"> {
  name?: string;
  size?: AvatarSize;
  shape?: AvatarShape;
  color?: string;
}

function AvatarFallback({
  name = "?",
  size = "md",
  shape = "square",
  color,
  className,
  ...props
}: AvatarFallbackProps) {
  const { bg, fg } = color ? { bg: color, fg: "text-avatar-1-fg" } : hashColor(name);
  return (
    <AvatarRoot size={size} shape={shape} aria-label={name} className={cn(bg, fg, className)} {...props}>
      <span className="select-none">{getInitial(name)}</span>
    </AvatarRoot>
  );
}

export { AvatarRoot, AvatarFallback, type AvatarSize, type AvatarShape };
