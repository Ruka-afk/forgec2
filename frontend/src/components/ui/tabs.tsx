"use client"

import { Tabs as TabsPrimitive } from "@base-ui/react/tabs"

import { cn } from "@/lib/utils"

function Tabs({ className, ...props }: TabsPrimitive.Root.Props) {
  return <TabsPrimitive.Root data-slot="tabs" className={cn("flex flex-col gap-4", className)} {...props} />
}

type TabsListProps = TabsPrimitive.List.Props & {
  variant?: "toolbar" | "sidebar"
}

function TabsList({ className, variant = "toolbar", ...props }: TabsListProps) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      className={cn(
        variant === "sidebar"
          ? "flex h-auto min-h-0 w-full flex-col items-stretch gap-1.5 overflow-visible border-0 bg-transparent p-0 text-muted-foreground shadow-none"
          : "inline-flex h-auto min-h-11 max-w-full items-center justify-start gap-0.5 overflow-x-auto rounded-lg border border-border/60 bg-muted/65 p-0.5 text-muted-foreground shadow-sm scrollbar-thin sm:h-9 sm:min-h-0 sm:p-1",
        className
      )}
      {...props}
    />
  )
}

function TabsTrigger({ className, ...props }: TabsPrimitive.Tab.Props) {
  return (
    <TabsPrimitive.Tab
      data-slot="tabs-trigger"
      className={cn(
        "inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-md px-3 py-2 text-(--fs-compact) font-medium transition-all outline-none hover:bg-background/55 hover:text-foreground focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 data-[selected]:bg-background data-[selected]:text-foreground data-[selected]:shadow-sm sm:min-h-0 sm:py-1.5",
        className
      )}
      {...props}
    />
  )
}

function TabsContent({ className, ...props }: TabsPrimitive.Panel.Props) {
  return (
    <TabsPrimitive.Panel
      data-slot="tabs-content"
      className={cn(
        "mt-0 outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50",
        className
      )}
      {...props}
    />
  )
}

export { Tabs, TabsList, TabsTrigger, TabsContent }
