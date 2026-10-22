
import { Inbox } from "lucide-react";
import { cn } from "@/lib/utils";

export function EmptyState({ icon, title, message, action, className, compact = false }: {
  icon?: React.ComponentType<{ className?: string }>;
  title: string;
  message?: string;
  action?: React.ReactNode;
  className?: string;
  /** compact: table-cell / panel-friendly variant — small icon chip, tight padding. */
  compact?: boolean;
}) {
  const IconComponent = icon || Inbox;
  // `whitespace-normal` is load-bearing: the empty state is routinely rendered
  // inside a `TableCell`, which sets `whitespace-nowrap` for data cells. That
  // inherited nowrap clipped the message ("Deploy your first implant to
  // establish a…") instead of wrapping it, on every table-backed page.
  if (compact) {
    return (
      <div className={cn("flex flex-col items-center justify-center gap-1.5 whitespace-normal py-6 text-center", className)}>
        <IconComponent className="size-5 text-muted-foreground/70" aria-hidden="true" />
        <p className="text-compact font-medium text-muted-foreground">{title}</p>
        {message && <p className="max-w-xs text-balance text-xs text-muted-foreground/80">{message}</p>}
        {action && <div className="mt-1">{action}</div>}
      </div>
    );
  }
  return (
    // Dense-console sizing: the full-page empty state used to reserve ~240px
    // (min-h-40 + md:py-20), which turned every empty panel into a wall of
    // whitespace on the dashboard and in card bodies.
    <div role="status" className={cn("flex min-h-28 flex-col items-center justify-center whitespace-normal py-8 text-center animate-fade-in sm:py-10 md:py-12", className)}>
      <div className="mb-3 flex size-11 items-center justify-center rounded-lg bg-muted/80 shadow-sm ring-1 ring-border/50" aria-hidden="true">
        <IconComponent className="size-6 text-muted-foreground/85" aria-hidden="true" />
      </div>
      <p className="mb-1 text-sm font-semibold text-foreground">{title}</p>
      {message && <p className="mb-3 max-w-sm text-balance text-compact leading-relaxed text-muted-foreground">{message}</p>}
      {action && <div>{action}</div>}
    </div>
  );
}
