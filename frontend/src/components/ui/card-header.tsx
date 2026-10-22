
import { cn } from "@/lib/utils";
import { IconBadge } from "@/components/ui/icon-badge";
import { Badge } from "@/components/ui/badge";
import type { Hue } from "@/lib/ui/statusStyles";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

export interface CardHeaderProps {
  /** Section icon, rendered as a muted tile with a hue-coloured glyph. */
  icon?: LucideIcon;
  /** Accent hue for the icon glyph. */
  tone?: Hue;
  title: ReactNode;
  /** Small muted line rendered under the title. */
  description?: ReactNode;
  /** Optional numeric badge shown next to the title (e.g. item count). */
  count?: number;
  /** Right-aligned controls (buttons, triggers). */
  action?: ReactNode;
  /** Apply the command-center left accent bar to the header. */
  accent?: boolean;
  /** Heading level for the title. */
  titleAs?: "h2" | "h3";
  className?: string;
}

/**
 * The single card title bar used by every panel in the app.
 *
 * `SectionCard` (dashboard, agent detail, campaign, …) and `CardHeaderRow`
 * (settings, automation, traffic, …) used to ship two different geometries:
 * a bare 24px icon with an inline description versus a 36px icon tile with a
 * stacked description. Both now render this component, so a card header looks
 * the same wherever it appears.
 */
export function CardHeader({
  icon: Icon,
  tone = "primary",
  title,
  description,
  count,
  action,
  accent = false,
  titleAs = "h2",
  className,
}: CardHeaderProps) {
  const Title = titleAs;
  return (
    <div
      data-slot="card-header"
      className={cn(
        "flex items-center gap-3 border-b border-border/65 bg-muted/20 px-3.5 py-2.5",
        accent && "panel-header-accent",
        className,
      )}
    >
      {Icon && (
        <IconBadge icon={Icon} color={tone} size="sm" className="bg-muted/70 shadow-none" />
      )}
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex min-w-0 items-center gap-2">
          <Title className="truncate text-sm font-semibold leading-tight text-foreground">
            {title}
          </Title>
          {typeof count === "number" && (
            <Badge variant={count > 0 ? "secondary" : "outline"} className="shrink-0 font-mono">
              {count}
            </Badge>
          )}
        </div>
        {description && (
          <p className="line-clamp-2 text-xs leading-snug text-muted-foreground">{description}</p>
        )}
      </div>
      {action && <div className="flex shrink-0 items-center gap-2">{action}</div>}
    </div>
  );
}
