import { type ReactNode } from "react";
import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";
import { FieldError } from "@/components/ui/field-error";

/** The id a field's error message carries, for the control's aria-describedby. */
export function fieldErrorId(controlId: string): string {
  return `${controlId}-error`;
}

export interface FieldProps {
  /** The field's label. */
  label: ReactNode;
  /** Marks the field required; the label renders the marker. */
  required?: boolean;
  /** Helper text under the control. */
  hint?: ReactNode;
  /** Error text under the control. */
  error?: ReactNode;
  /** id of the control: the label points at it and the error describes it. */
  htmlFor?: string;
  className?: string;
  children: ReactNode;
}

/**
 * One form field: label, control, hint and error, in that order and with one
 * spacing.
 *
 * The forms across the console used to spell the label, the gap and the
 * error out by hand every time - ten different label classNames, four
 * different gaps, and an error text that was usually not wired to the
 * control with aria-describedby. The primitive owns the geometry and the
 * accessibility wiring; a form only decides what the field means.
 */
export function Field({
  label,
  required,
  hint,
  error,
  htmlFor,
  className,
  children,
}: FieldProps) {
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <Label htmlFor={htmlFor} required={required}>
        {label}
      </Label>
      {children}
      {error ? (
        <FieldError id={htmlFor ? fieldErrorId(htmlFor) : undefined}>{error}</FieldError>
      ) : hint ? (
        <p
          id={htmlFor ? `${htmlFor}-hint` : undefined}
          className="text-micro-sm text-muted-foreground"
        >
          {hint}
        </p>
      ) : null}
    </div>
  );
}
