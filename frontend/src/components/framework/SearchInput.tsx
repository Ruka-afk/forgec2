"use client";

import { useId } from "react";
import { Input } from "@/components/ui/input";
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
}: {
  value: string;
  onChange: (next: string) => void;
  onClear?: () => void;
  placeholder?: string;
  className?: string;
  inputClassName?: string;
  label?: string;
}) {
  const id = useId();
  const { t } = useI18n();
  const clear = onClear ?? (() => onChange(""));
  return (
    <div className={cn("relative min-w-0", className)}>
      <Search
        className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground/60"
        aria-hidden="true"
      />
      <label htmlFor={id} className="sr-only">
        {label}
      </label>
      <Input
        id={id}
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        spellCheck={false}
        autoComplete="off"
        className={cn(
          "h-9 rounded-lg border-transparent bg-secondary/40 pl-8 pr-8 text-(--fs-compact) placeholder:text-muted-foreground/60 focus:bg-background dark:bg-secondary/30",
          inputClassName
        )}
      />
      {value && (
        <button
          type="button"
          onClick={clear}
          aria-label={t("search.clear")}
          className="absolute right-2 top-1/2 -translate-y-1/2 grid h-5 w-5 place-items-center rounded-md text-muted-foreground/60 transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      )}
    </div>
  );
}