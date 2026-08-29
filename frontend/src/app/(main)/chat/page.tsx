"use client";
import { useState, useEffect, useRef, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useWS } from "@/lib/wsContext";
import { formatTime } from "@/lib/utils";
import { PageHeader } from "@/components/ui/page-header";
import { WorkspaceShell } from "@/components/ui/workspace-shell";
import { DataState } from "@/components/ui/data-state";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Menu } from "lucide-react";
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

function ChannelRow({ channel, active, onSelect }: { channel: ChannelInfo; active: boolean; onSelect: () => void }) {
  return (
    <div key={channel.channel} role="button" tabIndex={0}
      className={`flex justify-between px-2.5 py-2 rounded-lg text-xs cursor-pointer transition-colors ${active ? "bg-primary text-primary-foreground" : "hover:bg-secondary text-muted-foreground"}`}
      onClick={onSelect}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onSelect(); } }}>
      <span># {channel.channel}</span>
      <span className="text-(--fs-xs-sm) text-muted-foreground">{channel.message_count}</span>
    </div>
  );
}

export default function ChatPage() {
  const { t } = useI18n();
  const [messages, setMessages] = useState<ChatMsg[]>([]);
  const [channels, setChannels] = useState<ChannelInfo[]>([{ channel: "general", message_count: 0, last_message: "", last_time: "" }]);
  const [currentChannel, setCurrentChannel] = useState("general");
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [connected, setConnected] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const historyAbortRef = useRef<AbortController | null>(null);
  const { connected: wsConnected, subscribe } = useWS();

  const loadHistory = useCallback(async (silent = false, signal?: AbortSignal) => {
    if (!silent) { setLoading(true); setError(null); }
    try {
      const data = await api.get(paths.chat.history(currentChannel), { signal });
      const msgs = (data.messages as ChatMsg[]) || [];
      setMessages(msgs.slice(-500));
    } catch (e) {
      if ((e as Error).name !== 'AbortError') {
        const msg = t("chat.load_history_failed");
        if (!silent) setError(msg);
        toast.error(msg);
      }
    }
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
        loadHistory(true, historyAbortRef.current?.signal);
      }
    });
    return unsub;
  }, [subscribe, currentChannel, wsConnected, loadHistory]);

  useEffect(() => {
    const controller = new AbortController();
    historyAbortRef.current = controller;
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
    <WorkspaceShell
      className="h-full"
      header={
        <PageHeader title={t("chat.title")} subtitle={t("chat.subtitle")} className="border-0 pb-0">
          <Badge variant={connected ? "success" : "destructive"}>
            {connected ? t("chat.connected") : t("chat.disconnected")}
          </Badge>
        </PageHeader>
      }
    >
      <div className="flex h-full min-h-0 flex-1 overflow-hidden">
        <aside className="hidden md:flex w-[200px] shrink-0 flex-col bg-card p-3 border-r border-border overflow-y-auto">
          <h4 className="text-xs font-semibold uppercase text-muted-foreground mb-2 m-0">{t("chat.channels")}</h4>
          {channels.map(c => (
            <ChannelRow key={c.channel} channel={c} active={c.channel === currentChannel} onSelect={() => setCurrentChannel(c.channel)} />
          ))}
        </aside>

        <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
          <SheetContent side="left" className="w-64 p-3">
            <h4 className="text-xs font-semibold uppercase text-muted-foreground mb-2 m-0">{t("chat.channels")}</h4>
            {channels.map(c => (
              <ChannelRow key={c.channel} channel={c} active={c.channel === currentChannel} onSelect={() => { setCurrentChannel(c.channel); setSidebarOpen(false); }} />
            ))}
          </SheetContent>
        </Sheet>

        <div className="flex-1 flex flex-col">
          <div className="px-4 py-2.5 border-b border-border bg-card flex items-center gap-2">
            <Button variant="ghost" size="icon-sm" onClick={() => setSidebarOpen(true)} className="md:hidden shrink-0" aria-label={t("chat.toggle_channels")}>
              <Menu className="size-4" />
            </Button>
            <h3 className="text-sm font-semibold text-foreground m-0"># {currentChannel}</h3>
          </div>
          <DataState
            loading={loading}
            error={error}
            onRetry={() => loadHistory()}
            empty={!loading && !error && messages.length === 0}
            emptyTitle={t("chat.no_messages")}
            loadingSkeleton={
              <div className="px-4 py-3 flex flex-col gap-2">
                {[1, 2, 3].map(i => (
                  <div key={i} className="px-2.5 py-1.5 rounded-lg bg-card">
                    <Skeleton className="h-3 w-24 mb-2" />
                    <Skeleton className="size-3/4" />
                  </div>
                ))}
              </div>
            }
          >
            <div className="flex-1 overflow-y-auto px-4 py-3 flex flex-col gap-2">
              {messages.map(m => (
                <div key={m.id} className="px-2.5 py-1.5 rounded-lg bg-card">
                  <span className="font-semibold text-xs text-primary mr-2">{m.username}</span>
                  <span className="text-(--fs-xs-sm) text-muted-foreground">{formatTime(m.created_at)}</span>
                  <div className="mt-1 text-sm text-foreground whitespace-pre-wrap break-words">{m.message}</div>
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>
          </DataState>
          <div className="flex gap-2 px-4 py-3 border-t border-border bg-card">
            <Input aria-label={t("chat.input_placeholder")} name="input-0" value={input} onChange={e => setInput(e.target.value)} onKeyDown={handleKeyDown}
              placeholder={t("chat.input_placeholder")}
              className="flex-1 h-10" />
            <Button onClick={sendMessage} disabled={!input.trim() || sending}
              >{sending ? t("chat.sending") : t("chat.send")}</Button>
          </div>
        </div>
      </div>
    </WorkspaceShell>
  );
}

