"use client";
import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { formatTime } from "@/lib/utils";
import { Trash2, RefreshCw, Wifi, WifiOff } from "lucide-react";

interface ExtC2Channel {
  id: number;
  type: string;
  channel_id: string;
  enabled: boolean;
  created_at: string;
}

interface ExtC2ChannelList {
  channels: ExtC2Channel[];
}

export default function ExtC2Section() {
  const { t } = useI18n();
  const [channels, setChannels] = useState<ExtC2Channel[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [formType, setFormType] = useState<"discord" | "slack" | "telegram">("discord");
  const [botToken, setBotToken] = useState("");
  const [channelId, setChannelId] = useState("");
  const [saving, setSaving] = useState(false);
  const { confirm, modal } = useConfirm();

  const fetchChannels = useCallback(async () => {
    try {
      const data: ExtC2ChannelList = await api.get(paths.extc2.configs);
      setChannels(data.channels || []);
    } catch { /* ignore */ }
    setLoading(false);
  }, []);

  useEffect(() => { fetchChannels(); }, [fetchChannels]);

  const handleSave = async () => {
    if (!botToken || !channelId) {
      toast.error(t("settings.toast.extc2_required"));
      return;
    }
    setSaving(true);
    try {
      const endpoint = formType === "discord" ? "/extc2/discord" : formType === "slack" ? "/extc2/slack" : "/extc2/telegram";
      const payload = formType === "telegram"
        ? { bot_token: botToken, chat_id: channelId }
        : { bot_token: botToken, channel_id: channelId };
      await api.postJson(endpoint, payload);
      toast.success(t("settings.toast.extc2_configured", { type: formType.charAt(0).toUpperCase() + formType.slice(1) }));
      setBotToken("");
      setChannelId("");
      setShowForm(false);
      fetchChannels();
    } catch {
      toast.error(t("settings.toast.channel_config_failed"));
    }
    setSaving(false);
  };

  const handleDelete = async (id: number) => {
    if (!(await confirm({ message: t("settings.extc2.delete_confirm") }))) return;
    try {
      await api.del(paths.extc2.config(id));
      toast.success(t("settings.toast.channel_removed"));
      fetchChannels();
    } catch { /* ignore */ }
  };

  return (
    <Card className="p-(--card-spacing) space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-semibold text-sm">{t("settings.extc2")}</h3>
          <p className="text-xs text-muted-foreground">{t("settings.extc2Desc")}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={fetchChannels} className="rounded-lg">
            <RefreshCw className="size-4" />
          </Button>
          <Button size="sm" onClick={() => setShowForm(!showForm)} className="rounded-lg">
            + {t("settings.extc2.addChannel")}
          </Button>
        </div>
      </div>

      {channels.length === 0 && !loading && (
        <p className="text-xs text-muted-foreground text-center py-4">{t("settings.extc2.noChannels")}</p>
      )}

      {modal}

      {channels.map(ch => (
        <div key={ch.id} className="flex items-center justify-between p-3 rounded-lg bg-muted/50">
          <div className="flex items-center gap-3">
            {ch.enabled ? <Wifi className="size-4 text-success" /> : <WifiOff className="size-4 text-muted-foreground" />}
            <div>
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="text-xs">{ch.type}</Badge>
                <span className="text-sm font-medium">{ch.channel_id}</span>
              </div>
              <p className="text-xs text-muted-foreground mt-0.5">{t("settings.extc2.created")} {formatTime(ch.created_at)}</p>
            </div>
          </div>
          <Button variant="ghost" size="sm" onClick={() => handleDelete(ch.id)} className="rounded-lg text-destructive hover:text-destructive">
            <Trash2 className="size-4" />
          </Button>
        </div>
      ))}

      {showForm && (
        <div className="space-y-3 p-4 rounded-lg border bg-muted/30">
          <div className="flex gap-2">
            <Button
              variant={formType === "discord" ? "default" : "outline"}
              size="sm"
              onClick={() => setFormType("discord")}
              className="rounded-lg"
            >
              Discord
            </Button>
            <Button
              variant={formType === "slack" ? "default" : "outline"}
              size="sm"
              onClick={() => setFormType("slack")}
              className="rounded-lg"
            >
              Slack
            </Button>
            <Button
              variant={formType === "telegram" ? "default" : "outline"}
              size="sm"
              onClick={() => setFormType("telegram")}
              className="rounded-lg"
            >
              Telegram
            </Button>
          </div>
          <div className="space-y-2">
            <Label required htmlFor="extc2-bot-token" className="text-xs">{t("settings.extc2.botToken")}</Label>
            <Input
              id="extc2-bot-token"
              type="password"
              value={botToken}
              onChange={e => setBotToken(e.target.value)}
              placeholder={t("settings.extc2.botTokenPlaceholder", { type: formType === "telegram" ? "Telegram" : formType === "discord" ? "Discord" : "Slack" })}
              className="h-8 text-xs"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="extc2-channel-id" className="text-xs">{formType === "telegram" ? t("settings.extc2.chatId") : t("settings.extc2.channelId")}</Label>
            <Input
              id="extc2-channel-id"
              value={channelId}
              onChange={e => setChannelId(e.target.value)}
              placeholder={formType === "telegram" ? t("settings.extc2.chatIdPlaceholder") : t("settings.extc2.channelId")}
              className="h-8 text-xs"
            />
          </div>
          <div className="flex gap-2">
            <Button size="sm" onClick={handleSave} disabled={saving} className="rounded-lg">
              {saving ? t("settings.extc2.saving") : t("settings.extc2.save")}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setShowForm(false)} className="rounded-lg">
              {t("settings.extc2.cancel")}
            </Button>
          </div>
        </div>
      )}
    </Card>
  );
}
