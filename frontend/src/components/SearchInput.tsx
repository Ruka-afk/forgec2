"use client";

import { useId } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import { Search, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";

/**
 * F1 blueprint — focused search field used across list pages. Stateless;
 * the parent owns `value`/`onChange` so every page keeps its debounce and
 * URL-sync logic.
 */
export function SearchInput({
  value,
  onChange,
  onClear,
  placeholder,
  className,
  inputClassName,
  label = "Search",
  id: idProp,
}: {
  value: string;
  onChange: (next: string) => void;
  onClear?: () => void;
  placeholder?: string;
  className?: string;
  inputClassName?: string;
  label?: string;
  id?: string;
}) {
  const genId = useId();
  const id = idProp || genId;
  const { t } = useI18n();
  const clear = onClear ?? (() => onChange(""));
  return (
    <div className={cn("relative min-w-0", className)}>
      <Search
        className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground/85"
        aria-hidden="true"
      />
      <Label htmlFor={id} className="sr-only">
        {label}
      </Label>
      <Input
        id={id}
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        spellCheck={false}
        autoComplete="off"
        className={cn(
          "h-9 rounded-lg border-transparent bg-secondary/40 pl-8 pr-8 text-(--fs-compact) placeholder:text-muted-foreground/85 focus:bg-background dark:bg-secondary/30",
          inputClassName
        )}
      />
      {value && (
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          onClick={clear}
          aria-label={t("search.clear")}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground/85 hover:text-foreground"
        >
          <X className="size-3.5" />
        </Button>
      )}
    </div>
  );
}