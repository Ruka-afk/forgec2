import { CardHeader, type CardHeaderProps } from "@/components/ui/card-header";

type CardHeaderRowProps = Omit<CardHeaderProps, "count" | "titleAs">;

/**
 * Unified card title bar used inside `overflow-hidden` cards.
 * Shares the SectionCard header geometry so both card systems read as one.
 */
export function CardHeaderRow(props: CardHeaderRowProps) {
  return <CardHeader {...props} />;
}
