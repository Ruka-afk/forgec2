"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { DEFAULT_SHORTCUTS, loadShortcuts, formatShortcut, matchShortcut } from "@/lib/shortcuts";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Keyboard } from "lucide-react";
import { useI18n } from "@/lib/i18n";

const groups = [
  { titleKey: "shortcuts.group_nav", keys: ["global_search", "refresh"] },
  { titleKey: "shortcuts.group_actions", keys: ["new_item", "save", "toggle_lock"] },
  { titleKey: "shortcuts.group_general", keys: ["show_shortcuts", "close_modal"] },
];

function ShortcutsHelpPanel({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const { t } = useI18n();
  const [shortcuts, setShortcuts] = useState(DEFAULT_SHORTCUTS);
  const isMac = typeof navigator !== "undefined" && /Mac/.test(navigator.platform);

  useEffect(() => {
    if (open) Promise.resolve().then(() => setShortcuts(loadShortcuts()));
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg" aria-label={t("a11y.shortcuts")}>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Keyboard aria-hidden="true" className="w-4 h-4" />
            {t("a11y.shortcuts")}
          </DialogTitle>
        </DialogHeader>
        <ScrollArea className="space-y-5 max-h-[60vh]">
          {groups.map((g) => (
            <div key={g.titleKey}>
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">{t(g.titleKey)}</h3>
              <div className="space-y-2">
                {g.keys.map((key) => {
                  const s = shortcuts[key];
                  if (!s) return null;
                  return (
                    <div key={key} className="flex items-center justify-between gap-3 text-sm">
                      <span className="text-muted-foreground">{t(s.descKey)}</span>
                      <kbd className="px-2 py-1 bg-secondary rounded-lg text-xs font-mono text-muted-foreground border border-border shrink-0">
                        {formatShortcut(s, isMac)}
                      </kbd>
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
          <p className="text-xs text-muted-foreground pt-2 border-t border-border">
            <Link href="/settings#section-shortcuts" className="text-primary hover:underline">
              {t("shortcuts.customize")}
            </Link>{" "}
            {t("shortcuts.in_settings")}
          </p>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  );
}

export default function ShortcutsHelp() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const s = loadShortcuts();
      if (matchShortcut(e, s.show_shortcuts)) {
        e.preventDefault();
        setOpen((v) => !v);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return <ShortcutsHelpPanel open={open} onOpenChange={setOpen} />;
}

export function ShortcutsHelpButton() {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);

  return (
    <>
      <Tooltip>
        <TooltipTrigger render={<Button onClick={() => setOpen(true)} variant="ghost" size="sm" className="hidden md:flex text-xs" />}>
          <Keyboard aria-hidden="true" className="w-4 h-4" />
          <span>{t("common.shortcuts")}</span>
        </TooltipTrigger>
        <TooltipContent>{t("common.shortcuts_hint")}</TooltipContent>
      </Tooltip>
      <ShortcutsHelpPanel open={open} onOpenChange={setOpen} />
    </>
  );
}