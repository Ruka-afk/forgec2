"use client"

import * as React from "react"

import { cn } from "@/lib/utils"
import { Inbox } from "lucide-react"

function Table({ className, ...props }: React.ComponentProps<"table">) {
  return (
    <div
      data-slot="table-container"
      className="relative w-full overflow-x-auto rounded-lg bg-card scrollbar-thin [scrollbar-gutter:stable]"
    >
      <table
        data-slot="table"
        className={cn("w-full caption-bottom text-sm", className)}
        {...props}
      />
    </div>
  )
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("sticky top-0 z-10 bg-muted/90 [&_tr]:border-b", className)}
      {...props}
    />
  )
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  )
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        "border-t bg-muted/50 font-medium [&>tr]:last:border-b-0",
        className
      )}
      {...props}
    />
  )
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "border-b border-border/70 transition-colors duration-150 hover:bg-primary/5 has-aria-expanded:bg-muted/70 data-[state=selected]:bg-primary/8",
        className
      )}
      {...props}
    />
  )
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "h-10 px-4 text-left align-middle font-semibold whitespace-nowrap text-(--fs-micro-sm) uppercase tracking-(--tracking-sublabel) text-muted-foreground [&:has([role=checkbox])]:pr-0 [&>svg]:size-3.5",
        className
      )}
      {...props}
    />
  )
}

function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn(
        "px-4 py-2.5 align-middle whitespace-nowrap text-sm [&:has([role=checkbox])]:pr-0",
        className
      )}
      {...props}
    />
  )
}

function TableCaption({
  className,
  ...props
}: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

function TableEmptyState({
  colSpan,
  message,
  icon: Icon = Inbox,
  className,
}: {
  colSpan: number;
  message?: React.ReactNode;
  icon?: React.ComponentType<{ className?: string }>;
  className?: string;
}) {
  return (
    <TableRow>
      <TableCell
        colSpan={colSpan}
        className={cn("py-14 sm:py-16 text-center text-muted-foreground", className)}
      >
        <div className="flex flex-col items-center gap-2">
          {Icon && (
            <span className="grid size-9 place-items-center rounded-lg bg-muted text-muted-foreground/80">
              <Icon className="size-4" />
            </span>
          )}
          {message && <span className="text-(--fs-compact)">{message}</span>}
        </div>
      </TableCell>
    </TableRow>
  );
}

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
  TableEmptyState,
}
