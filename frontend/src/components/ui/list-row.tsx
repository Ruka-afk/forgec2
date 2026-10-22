import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

export interface ListRowProps {
  /** Internal route; without it the row renders as a plain container. */
  to?: string;
  /** Left slot rendered before the title (status dot, badge, icon). */
  leading?: ReactNode;
  /** Primary line. */
  title: ReactNode;
  /** Secondary line under the title. */
  subtitle?: ReactNode;
  /** Right slot (timestamp, count, status). */
  meta?: ReactNode;
  /** Extra classes for the right slot. */
  metaClassName?: string;
  /** Extra classes for the row. */
  className?: string;
}

/**
 * The console's list row: a left slot, a two-line text block and a right
 * slot, on the shared hover surface.
 *
 * The dashboard panels used to copy this geometry five times, each with its
 * own padding and hover tint; a panel that looked like a list therefore had
 * to be re-styled by hand. One primitive keeps the rows identical wherever
 * they appear.
 */
export function ListRow({
  to,
  leading,
  title,
  subtitle,
  meta,
  metaClassName,
  className,
}: ListRowProps) {
  const body = (
    <>
      {leading && <span className="flex shrink-0 items-center gap-2">{leading}</span>}
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-2">{title}</span>
        {subtitle && (
          <span className="mt-0.5 block truncate text-micro-sm text-muted-foreground">{subtitle}</span>
        )}
      </span>
      {meta && (
        <span className={cn("shrink-0 text-micro-sm text-muted-foreground", metaClassName)}>{meta}</span>
      )}
    </>
  );
  const classes = cn(
    "flex items-center justify-between gap-2 px-(--card-spacing) py-2 transition-colors hover:bg-(--surface-hover)",
    className,
  );
  if (to) {
    return (
      <Link to={to} className={classes}>
        {body}
      </Link>
    );
  }
  return <div className={classes}>{body}</div>;
}
