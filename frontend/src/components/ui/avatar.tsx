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

function hashColor(name: string): string {
  const palette = [
    "bg-[var(--avatar-1)]",
    "bg-[var(--avatar-2)]",
    "bg-[var(--avatar-3)]",
    "bg-[var(--avatar-4)]",
    "bg-[var(--avatar-5)]",
    "bg-[var(--avatar-6)]",
    "bg-[var(--avatar-7)]",
    "bg-[var(--avatar-8)]",
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
      className={cn(
        "relative inline-flex shrink-0 items-center justify-center font-bold text-white overflow-hidden",
        AVATAR_SIZES[size],
        AVATAR_SHAPES[shape],
        className
      )}
      {...props}
    />
  )
);
AvatarRoot.displayName = "AvatarRoot";

interface AvatarImageProps extends React.ComponentProps<"img"> {
  size?: AvatarSize;
  shape?: AvatarShape;
  alt?: string;
}

function AvatarImage({ className, size = "md", shape = "square", alt = "", ...props }: AvatarImageProps) {
  return (
    <img
      className={cn(
        "aspect-square h-full w-full object-cover",
        AVATAR_SIZES[size],
        AVATAR_SHAPES[shape],
        className
      )}
      alt={alt}
      {...props}
    />
  );
}

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
  return (
    <AvatarRoot size={size} shape={shape} className={cn(color || hashColor(name), className)} {...props}>
      <span className="select-none">{getInitial(name)}</span>
    </AvatarRoot>
  );
}

export { AvatarRoot, AvatarImage, AvatarFallback, type AvatarSize, type AvatarShape };
