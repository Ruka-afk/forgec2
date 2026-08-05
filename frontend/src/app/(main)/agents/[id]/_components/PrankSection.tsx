"use client";

import { useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogClose } from "@/components/ui/dialog";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Globe, Image, MessageSquare, Music, Power, RotateCcw, StickyNote, Volume2, MousePointer2, Disc } from "lucide-react";

interface PrankAction {
  id: string;
  icon: React.ReactNode;
  labelKey: string;
  needsInput: boolean;
  inputLabel?: string;
  inputPlaceholder?: string;
  route: string;
  buildPayload?: (value: string) => Record<string, string>;
}

const PRANK_ACTIONS: PrankAction[] = [
  // eslint-disable-next-line jsx-a11y/alt-text -- lucide-react Image icon, not an HTML img
  { id: "wallpaper", icon: <Image className="w-5 h-5" aria-hidden="true" />, labelKey: "agents.prank_wallpaper", needsInput: true, inputLabel: "agents.prank_wallpaper_hint", inputPlaceholder: "https://example.com/image.jpg", route: "/prank/wallpaper" },
  { id: "msgbox", icon: <MessageSquare className="w-5 h-5" />, labelKey: "agents.prank_msgbox", needsInput: true, inputLabel: "agents.prank_msgbox_hint", inputPlaceholder: "Hello from ForgeC2!", route: "/prank/msgbox", buildPayload: (v) => ({ command: v, shell: "ForgeC2" }) },
  { id: "sound", icon: <Music className="w-5 h-5" />, labelKey: "agents.prank_sound", needsInput: true, inputLabel: "agents.prank_sound_hint", inputPlaceholder: "C:\\Windows\\Media\\chord.wav", route: "/prank/sound" },
  { id: "open_url", icon: <Globe className="w-5 h-5" />, labelKey: "agents.prank_open_url", needsInput: true, inputLabel: "agents.prank_url_hint", inputPlaceholder: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", route: "/prank/open_url" },
  { id: "screen_rotate", icon: <RotateCcw className="w-5 h-5" />, labelKey: "agents.prank_screen_rotate", needsInput: false, route: "/prank/screen_rotate" },
  { id: "cdrom", icon: <Disc className="w-5 h-5" />, labelKey: "agents.prank_cdrom", needsInput: true, inputLabel: "agents.prank_cdrom_hint", inputPlaceholder: "open", route: "/prank/cdrom" },
  { id: "notepad", icon: <StickyNote className="w-5 h-5" />, labelKey: "agents.prank_notepad", needsInput: true, inputLabel: "agents.prank_notepad_hint", inputPlaceholder: "5", route: "/prank/notepad" },
  { id: "lock", icon: <Power className="w-5 h-5" />, labelKey: "agents.prank_lock", needsInput: false, route: "/prank/lock" },
  { id: "volume", icon: <Volume2 className="w-5 h-5" />, labelKey: "agents.prank_volume", needsInput: true, inputLabel: "agents.prank_volume_hint", inputPlaceholder: "0", route: "/prank/volume" },
  { id: "cursor", icon: <MousePointer2 className="w-5 h-5" />, labelKey: "agents.prank_cursor", needsInput: false, route: "/prank/cursor" },
];

interface PrankSectionProps {
  agentId: string;
  online: boolean;
}

export default function PrankSection({ agentId, online }: PrankSectionProps) {
  const { t } = useI18n();
  const [selectedAction, setSelectedAction] = useState<PrankAction | null>(null);
  const [inputValue, setInputValue] = useState("");
  const [sending, setSending] = useState(false);

  const handleExecute = async () => {
    if (!selectedAction) return;
    setSending(true);
    try {
      const payload = selectedAction.buildPayload
        ? selectedAction.buildPayload(inputValue)
        : { command: inputValue };
      await api.postJson(paths.agents.cmd(agentId, selectedAction.route), payload);
      toast.success(t("agents.prank_sent").replace("{action}", t(selectedAction.labelKey)));
      setSelectedAction(null);
      setInputValue("");
    } catch {
      toast.error(t("agents.prank_failed"));
    }
    setSending(false);
  };

  const handleQuickExecute = async (action: PrankAction) => {
    setSending(true);
    try {
      await api.postJson(paths.agents.cmd(agentId, action.route), { command: "" });
      toast.success(t("agents.prank_sent").replace("{action}", t(action.labelKey)));
    } catch {
      toast.error(t("agents.prank_failed"));
    }
    setSending(false);
  };

  return (
    <Card className="mb-4 gap-0">
      <div className="px-4 py-3 border-b border-border">
        {/* eslint-disable-next-line jsx-a11y/alt-text -- lucide-react Image icon, not an HTML img */}
        <h3 className="text-sm font-semibold text-foreground"><Image className="w-3.5 h-3.5" aria-hidden="true" />{t("agents.prank_title")}</h3>
        <p className="text-(--fs-xs-sm) text-amber-700 dark:text-amber-300 mt-1">{t("agents.prank_honesty")}</p>
      </div>
      <div className="p-3">
        <div className="grid grid-cols-3 sm:grid-cols-5 gap-3">
          {PRANK_ACTIONS.map((action) => (
            <Button
              key={action.id}
              variant="secondary"
              className="flex flex-col items-center gap-1.5 h-auto py-3 text-xs"
              disabled={!online || sending}
              onClick={() => {
                if (action.needsInput) {
                  setInputValue("");
                  setSelectedAction(action);
                } else {
                  handleQuickExecute(action);
                }
              }}
            >
              {action.icon}
              <span className="truncate w-full text-center">{t(action.labelKey)}</span>
            </Button>
          ))}
        </div>
      </div>

      <Dialog open={!!selectedAction} onOpenChange={(open) => { if (!open) { setSelectedAction(null); setInputValue(""); } }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>{selectedAction ? t(selectedAction.labelKey) : ""}</DialogTitle>
          </DialogHeader>
          {selectedAction?.needsInput && (
            <div className="space-y-2">
              <label htmlFor="prank-input" className="text-xs text-muted-foreground">{selectedAction.inputLabel ? t(selectedAction.inputLabel) : ""}</label>
              <Input
                id="prank-input"
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                placeholder={selectedAction.inputPlaceholder}
                autoFocus
                onKeyDown={(e) => { if (e.key === "Enter") handleExecute(); }}
              />
            </div>
          )}
          <DialogFooter>
            <DialogClose render={<Button variant="ghost" size="sm" />}>{t("agents.prank_cancel")}</DialogClose>
            <Button size="sm" onClick={handleExecute} disabled={sending}>
              {sending ? t("agents.prank_sending") : t("agents.prank_execute")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
