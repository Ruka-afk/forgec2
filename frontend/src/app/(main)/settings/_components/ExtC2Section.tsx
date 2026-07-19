"use client";
import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Trash2, RefreshCw, Wifi, WifiOff } from "lucide-react";

interface ExtC2Channel {
  id: number;
  type: string;
  channel_id: string;
  enabled: boolean;
  created_at: string;
}

export default function ExtC2Section() {
  const { t } = useI18n();
  const [channels, setChannels] = useState<ExtC2Channel[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [formType, setFormType] = useState<"discord" | "slack">("discord");
  const [botToken, setBotToken] = useState("");
  const [channelId, setChannelId] = useState("");
  const [saving, setSaving] = useState(false);

  const fetchChannels = useCallback(async () => {
    try {
      const data = await api.get("/extc2/configs") as any;
      setChannels(data?.channels || []);
    } catch { /* ignore */ }
    setLoading(false);
  }, []);

  useEffect(() => { fetchChannels(); }, [fetchChannels]);

  const handleSave = async () => {
    if (!botToken || !channelId) {
      toast.error("Bot token and channel ID are required");
      return;
    }
    setSaving(true);
    try {
      const endpoint = formType === "discord" ? "/extc2/discord" : "/extc2/slack";
      await api.post(endpoint, { bot_token: botToken, channel_id: channelId } as Record<string, string>);
      toast.success(`${formType.charAt(0).toUpperCase() + formType.slice(1)} External C2 configured`);
      setBotToken("");
      setChannelId("");
      setShowForm(false);
      fetchChannels();
    } catch (err) {
      toast.error("Failed to configure channel");
    }
    setSaving(false);
  };

  const handleDelete = async (id: number) => {
    try {
      await api.del(`/extc2/configs/${id}`);
      toast.success("Channel removed");
      fetchChannels();
    } catch { /* ignore */ }
  };

  return (
    <Card className="p-4 sm:p-5 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-semibold text-sm">{t("settings.extc2")}</h3>
          <p className="text-xs text-muted-foreground">{t("settings.extc2Desc")}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={fetchChannels} className="rounded-xl">
            <RefreshCw className="w-4 h-4" />
          </Button>
          <Button size="sm" onClick={() => setShowForm(!showForm)} className="rounded-xl">
            + Add Channel
          </Button>
        </div>
      </div>

      {channels.length === 0 && !loading && (
        <p className="text-xs text-muted-foreground text-center py-4">No External C2 channels configured</p>
      )}

      {channels.map(ch => (
        <div key={ch.id} className="flex items-center justify-between p-3 rounded-xl bg-muted/50">
          <div className="flex items-center gap-3">
            {ch.enabled ? <Wifi className="w-4 h-4 text-green-500" /> : <WifiOff className="w-4 h-4 text-muted-foreground" />}
            <div>
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="text-xs">{ch.type}</Badge>
                <span className="text-sm font-medium">{ch.channel_id}</span>
              </div>
              <p className="text-xs text-muted-foreground mt-0.5">Created {new Date(ch.created_at).toLocaleDateString()}</p>
            </div>
          </div>
          <Button variant="ghost" size="sm" onClick={() => handleDelete(ch.id)} className="rounded-xl text-destructive hover:text-destructive">
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      ))}

      {showForm && (
        <div className="space-y-3 p-4 rounded-xl border bg-muted/30">
          <div className="flex gap-2">
            <Button
              variant={formType === "discord" ? "default" : "outline"}
              size="sm"
              onClick={() => setFormType("discord")}
              className="rounded-xl"
            >
              Discord
            </Button>
            <Button
              variant={formType === "slack" ? "default" : "outline"}
              size="sm"
              onClick={() => setFormType("slack")}
              className="rounded-xl"
            >
              Slack
            </Button>
          </div>
          <div className="space-y-2">
            <Label className="text-xs">Bot Token</Label>
            <Input
              type="password"
              value={botToken}
              onChange={e => setBotToken(e.target.value)}
              placeholder={`${formType === "discord" ? "Discord" : "Slack"} bot token`}
              className="h-8 text-xs"
            />
          </div>
          <div className="space-y-2">
            <Label className="text-xs">Channel ID</Label>
            <Input
              value={channelId}
              onChange={e => setChannelId(e.target.value)}
              placeholder="Channel ID"
              className="h-8 text-xs"
            />
          </div>
          <div className="flex gap-2">
            <Button size="sm" onClick={handleSave} disabled={saving} className="rounded-xl">
              {saving ? "Saving..." : "Save"}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setShowForm(false)} className="rounded-xl">
              Cancel
            </Button>
          </div>
        </div>
      )}
    </Card>
  );
}
