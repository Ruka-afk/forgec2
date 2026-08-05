"use client";
import { useState, useEffect, useRef, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useWS } from "@/lib/wsContext";
import { formatTime } from "@/lib/utils";
import { PageHeader, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { useI18n } from "@/lib/i18n";

interface ChatMsg {
  id: number;
  username: string;
  message: string;
  channel: string;
  created_at: string;
}

interface ChannelInfo {
  channel: string;
  message_count: number;
  last_message: string;
  last_time: string;
}

export default function ChatPage() {
  const { t } = useI18n();
  const [messages, setMessages] = useState<ChatMsg[]>([]);
  const [channels, setChannels] = useState<ChannelInfo[]>([{ channel: "general", message_count: 0, last_message: "", last_time: "" }]);
  const [currentChannel, setCurrentChannel] = useState("general");
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [connected, setConnected] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const { connected: wsConnected, subscribe } = useWS();

  const loadHistory = useCallback(async (silent = false, signal?: AbortSignal) => {
    if (!silent) setLoading(true);
    try {
      const data = await api.get(paths.chat.history(currentChannel), { signal });
      const msgs = (data.messages as ChatMsg[]) || [];
      setMessages(msgs.slice(-500));
    } catch (e) { if ((e as Error).name !== 'AbortError') { setMessages([]); toast.error(t("chat.load_history_failed")); } }
    if (!silent) setLoading(false);
  }, [currentChannel, t]);

  const loadChannels = useCallback(async (signal?: AbortSignal) => {
    try {
      const data = await api.get(paths.chat.channels, { signal });
      if ((data.channels as ChannelInfo[])?.length > 0) setChannels(data.channels as ChannelInfo[]);
    } catch (e) { if ((e as Error).name !== 'AbortError') toast.error(t("chat.load_channels_failed")); }
  }, [t]);

  useEffect(() => {
    setConnected(wsConnected);
    const unsub = subscribe((msg: { type: string; channel?: string }) => {
      if (msg.type === "chat_message" && (msg.channel === currentChannel || !msg.channel)) {
        loadHistory(true);
      }
    });
    return unsub;
  }, [subscribe, currentChannel, wsConnected, loadHistory]);

  useEffect(() => {
    const controller = new AbortController();
    loadHistory(false, controller.signal);
    loadChannels(controller.signal);
    return () => controller.abort();
  }, [currentChannel, loadHistory, loadChannels]);

  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: "smooth" }); }, [messages]);

  async function sendMessage() {
    if (!input.trim()) return;
    const msg = input.trim();
    setInput("");
    setSending(true);
    try {
      const data = await api.postJson(paths.chat.send, { message: msg, channel: currentChannel });
      if (!data.success) { toast.error((data.error as string) || t("chat.send_failed")); setInput(msg); return; }
      loadHistory(true);
      loadChannels();
    } catch { toast.error(t("chat.connect_failed")); setInput(msg); }
    setSending(false);
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendMessage(); }
  }

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up flex flex-col h-[calc(100vh-100px)]">
      <PageHeader title={t("chat.title")} subtitle={t("chat.subtitle")}>
        <Badge variant={connected ? "success" : "destructive"}>
          {connected ? t("chat.connected") : t("chat.disconnected")}
        </Badge>
      </PageHeader>
      <div className="flex flex-1 gap-0 rounded-xl border border-border overflow-hidden">
        <div className="w-[200px] shrink-0 bg-card p-3 border-r border-border overflow-y-auto">
          <h4 className="text-xs font-semibold uppercase text-muted-foreground mb-2 m-0">{t("chat.channels")}</h4>
          {channels.map(c => (
            <div key={c.channel} role="button" tabIndex={0}
              className={`flex justify-between px-2.5 py-2 rounded-lg text-xs cursor-pointer transition-colors ${c.channel === currentChannel ? "bg-primary text-primary-foreground" : "hover:bg-secondary text-muted-foreground"}`}
              onClick={() => setCurrentChannel(c.channel)}
              onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setCurrentChannel(c.channel); } }}>
              <span># {c.channel}</span>
              <span className="text-(--fs-xs-sm) text-muted-foreground">{c.message_count}</span>
            </div>
          ))}
        </div>
        <div className="flex-1 flex flex-col">
          <div className="px-4 py-2.5 border-b border-border bg-card">
            <h3 className="text-sm font-semibold text-foreground m-0"># {currentChannel}</h3>
          </div>
          <div className="flex-1 overflow-y-auto px-4 py-3 flex flex-col gap-2">
            {loading ? (
              <div className="flex items-center justify-center py-12">
                <Spinner />
              </div>
            ) : messages.length === 0 ? (
              <div className="text-center py-12 text-sm text-muted-foreground">{t("chat.no_messages")}</div>
            ) : (
              messages.map(m => (
                <div key={m.id} className="px-2.5 py-1.5 rounded-lg bg-card">
                  <span className="font-semibold text-xs text-primary mr-2">{m.username}</span>
                  <span className="text-(--fs-xs-sm) text-muted-foreground">{formatTime(m.created_at)}</span>
                  <div className="mt-1 text-sm text-foreground whitespace-pre-wrap break-words">{m.message}</div>
                </div>
              ))
            )}
            <div ref={messagesEndRef} />
          </div>
          <div className="flex gap-2 px-4 py-3 border-t border-border bg-card">
            <Input aria-label="Type a message... (Enter to send)" name="input-0" value={input} onChange={e => setInput(e.target.value)} onKeyDown={handleKeyDown}
              placeholder={t("chat.input_placeholder")}
              className="flex-1 h-10" />
            <Button onClick={sendMessage} disabled={!input.trim() || sending}
              >{sending ? t("chat.sending") : t("chat.send")}</Button>
          </div>
        </div>
      </div>
    </div>
  );
}

