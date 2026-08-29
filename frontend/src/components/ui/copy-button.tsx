"use client";

import { useEffect, useRef, useState } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Check, Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";

export function CopyButton({ text, label, title, className, size = "icon-xs", children, onError }: {
  text: string;
  label?: string;
  title?: string;
  className?: string;
  size?: "icon-xs" | "xs" | "sm";
  children?: React.ReactNode | ((copied: boolean) => React.ReactNode);
  onError?: () => void;
}) {
  const { t } = useI18n();
  const [copied, setCopied] = useState(false);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);
  // G9 fix: add execCommand fallback for HTTP (non-secure context) where
  // navigator.clipboard is unavailable, plus a toast fallback on failure.
  const copy = async () => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        const ta = document.createElement("textarea");
        ta.value = text;
        ta.style.position = "fixed";
        ta.style.left = "-9999px";
        document.body.appendChild(ta);
        ta.select();
        document.execCommand("copy");
        document.body.removeChild(ta);
      }
      if (mountedRef.current) setCopied(true);
      setTimeout(() => { if (mountedRef.current) setCopied(false); }, 1500);
    } catch {
      onError?.();
      toast.error(t("common.copy_failed"));
    }
  };

  const defaultIcon = copied
    ? <Check className="size-2.5 text-success" />
    : <Copy className="size-2.5" />;

  return (
    <Tooltip>
      <TooltipTrigger render={<Button variant="ghost" size={size} onClick={copy} className={className || "ml-1 text-muted-foreground/100 hover:text-foreground"} aria-label={t("common.copy_value").replace("{value}", label || "value")} />}>
        {typeof children === "function" ? children(copied) : (children ?? defaultIcon)}
      </TooltipTrigger>
      <TooltipContent>{title || t("common.copy_value").replace("{value}", label || "value")}</TooltipContent>
    </Tooltip>
  );
}
