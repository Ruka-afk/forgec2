"use client"

import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DOUBLE_CLICK_MESSAGES } from "@/lib/easter-egg-quotes"
import { useState, useEffect } from "react"
import { useI18n } from "@/lib/i18n"

interface EasterEggPopupProps {
  open: boolean
  onClose: () => void
  hostname?: string
}

export function EasterEggPopup({ open, onClose, hostname }: EasterEggPopupProps) {
  const [message, setMessage] = useState("")
  const { t } = useI18n()

  useEffect(() => {
    if (open) {
      setMessage(DOUBLE_CLICK_MESSAGES[Math.floor(Math.random() * DOUBLE_CLICK_MESSAGES.length)])
    }
  }, [open])

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="w-80 gap-0">
        <DialogHeader>
          <DialogTitle className="sr-only">{t("agents.easter_egg_title")}</DialogTitle>
        </DialogHeader>
        <div className="text-center">
          <div className="text-4xl mb-3">🥚</div>
          <h3 className="text-sm font-bold text-foreground mb-2">{t("agents.easter_egg_title")}</h3>
          <p className="text-xs text-muted-foreground leading-relaxed">{message}</p>
          {hostname && (
            <p className="text-(--font-size-micro-sm) text-muted-foreground/50 mt-3">
              {t("agents.easter_egg_target")}: {hostname}
            </p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
