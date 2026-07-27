"use client";

import { useState } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Check, Copy } from "lucide-react";
import { Button } from "@/components/ui/button";

export function CopyButton({ text, label, title, className, size = "icon-xs", children, onError }: {
  text: string;
  label?: string;
  title?: string;
  className?: string;
  size?: "icon-xs" | "xs" | "sm";
  children?: React.ReactNode | ((copied: boolean) => React.ReactNode);
  onError?: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      onError?.();
    }
  };

  const defaultIcon = copied
    ? <Check className="w-2.5 h-2.5 text-emerald-500" />
    : <Copy className="w-2.5 h-2.5" />;

  return (
    <Tooltip>
      <TooltipTrigger render={<Button variant="ghost" size={size} onClick={copy} className={className || "ml-1 text-muted-foreground/70 hover:text-foreground"} aria-label={`Copy ${label || "value"}`} />}>
        {typeof children === "function" ? children(copied) : (children ?? defaultIcon)}
      </TooltipTrigger>
      <TooltipContent>{title || `Copy ${label || "value"}`}</TooltipContent>
    </Tooltip>
  );
}
