"use client";

type SpinnerSize = "xs" | "sm" | "md" | "lg" | "xl";

const SPINNER_SIZES: Record<SpinnerSize, string> = {
  xs: "size-3 border",
  sm: "size-4 border-2",
  md: "size-8 border-2",
  lg: "size-10 border-[3px]",
  xl: "size-12 border-[3px]",
};

const SPINNER_COLORS: Record<string, string> = {
  indigo: "border-primary border-t-transparent",
  white: "border-white border-t-transparent",
  blue: "border-info border-t-transparent",
  emerald: "border-success border-t-transparent",
  red: "border-destructive border-t-transparent",
};

export function Spinner({ size = "md", color = "indigo", className = "", label }: { size?: SpinnerSize; color?: string; className?: string; label?: string }) {
  return (
    <div
      role={label ? "status" : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
      className={`animate-spin rounded-full ${SPINNER_SIZES[size]} ${SPINNER_COLORS[color] || SPINNER_COLORS.indigo} ${className}`}
    />
  );
}

export function PageSpinner({ size = "md" }: { size?: SpinnerSize }) {
  return (
    <div className="flex items-center justify-center py-20">
      <Spinner size={size} />
    </div>
  );
}
