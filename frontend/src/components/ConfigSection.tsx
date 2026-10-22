
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import { CardHeader } from "@/components/ui/card-header";
import type { Hue } from "@/lib/ui/statusStyles";
import type { LucideIcon } from "lucide-react";

/**
 * F6 blueprint — grouped settings/management section card (title + optional
 * icon + description + actions, body, optional footer).
 */
export function ConfigSection({
  title,
  description,
  icon,
  tone,
  actions,
  children,
  footer,
  className,
  bodyClassName,
}: {
  title: ReactNode;
  description?: ReactNode;
  icon?: LucideIcon;
  /** Accent hue for the header icon. */
  tone?: Hue;
  actions?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <Card className={cn("gap-0 overflow-hidden py-0 shadow-1", className)}>
      <CardHeader icon={icon} tone={tone} title={title} description={description} action={actions} />
      {children && <CardContent className={cn("p-(--card-spacing)", bodyClassName)}>{children}</CardContent>}
      {footer && <CardFooter className="justify-between">{footer}</CardFooter>}
    </Card>
  );
}
