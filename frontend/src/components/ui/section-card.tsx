
import { Link } from "react-router-dom";
import { ChevronDown } from "lucide-react";
import { Card } from "@/components/ui/card";
import { CardHeader } from "@/components/ui/card-header";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";
import type { Hue } from "@/lib/ui/statusStyles";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

interface SectionCardProps {
  title: ReactNode;
  /** Section icon, rendered as a muted tile in the header. */
  icon?: LucideIcon;
  /** Accent hue for the icon glyph. */
  tone?: Hue;
  /** Optional numeric badge shown next to the title (e.g. item count). */
  count?: number;
  /** Small muted line rendered under the title. */
  description?: ReactNode;
  /** Right-aligned controls (buttons, triggers). */
  action?: ReactNode;
  /** Renders a "view all" link on the right. */
  href?: string;
  linkLabel?: string;
  /** Apply the command-center left accent bar to the header. */
  headerAccent?: boolean;
  /** Make the header a collapse trigger. */
  collapsible?: boolean;
  defaultOpen?: boolean;
  /** Controlled open state: when defined, Base UI ignores defaultOpen —
   *  required for parent-driven toggles (e.g. a "Load" action button that
   *  must open the panel programmatically). */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  className?: string;
  children: ReactNode;
}

/**
 * Unified panel/section container used across the dashboard and agent detail
 * page. Replaces the hand-rolled `Card + CardHeader` boilerplate that was
 * duplicated (with drifting styling) across ~8 call sites.
 */
export function SectionCard({
  title,
  icon,
  tone,
  count,
  description,
  action,
  href,
  linkLabel,
  headerAccent = false,
  collapsible = false,
  defaultOpen = true,
  open,
  onOpenChange,
  className,
  children,
}: SectionCardProps) {
  const headerAction = (
    <>
      {action}
      {collapsible && <ChevronDown className="size-3.5 text-muted-foreground" />}
      {href && linkLabel && (
        // `-my-2 py-2` grows the hit area to 32px without changing the
        // header height; a 16px-tall link is not a viable touch target.
        <Link
          to={href}
          className="-my-2 inline-flex min-h-8 items-center rounded-md px-1.5 text-xs text-primary transition-colors hover:bg-primary/8 hover:underline"
        >
          {linkLabel}
        </Link>
      )}
    </>
  );

  const header = (
    <CardHeader
      icon={icon}
      tone={tone}
      title={title}
      description={description}
      count={count}
      accent={headerAccent}
      action={
        action || collapsible || (href && linkLabel) ? headerAction : undefined
      }
    />
  );

  if (collapsible) {
    // Pass `open` only when the caller controls it: Base UI flips to
    // controlled mode the moment the prop is defined (even undefined via
    // spread), so conditionally include it.
    const collapsibleProps = open !== undefined ? { open, onOpenChange } : { defaultOpen, onOpenChange };
    return (
      <Card className={cn("gap-0 overflow-hidden py-0", className)}>
        <Collapsible {...collapsibleProps}>
          <CollapsibleTrigger className="w-full text-left">{header}</CollapsibleTrigger>
          <CollapsibleContent>{children}</CollapsibleContent>
        </Collapsible>
      </Card>
    );
  }

  return (
    <Card className={cn("gap-0 overflow-hidden py-0", className)}>
      {header}
      {children}
    </Card>
  );
}
