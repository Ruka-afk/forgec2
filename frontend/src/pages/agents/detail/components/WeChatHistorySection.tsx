import { memo, useEffect, useMemo, useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { CopyButton } from "@/components/ui/copy-button";
import { toast } from "sonner";
import { Download, MessageSquare, Search } from "lucide-react";
import { downloadJSON, downloadText } from "@/lib/download";
import { usePersistedState } from "@/lib/hooks/usePersistedState";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import {
  filterWeChatMessages,
  groupWeChatConversations,
  parseWeChatHistoryWithMeta,
  sortWeChatMessages,
  toWeChatCSV,
  type WeChatSenderFilter,
  type WeChatSortDirection,
} from "./wechat-history";
import { useCollectTask } from "./useCollectTask";

interface WeChatHistorySectionProps {
  agentId: string;
  online: boolean;
}

export default memo(function WeChatHistorySection({ agentId, online }: WeChatHistorySectionProps) {
  const { t } = useI18n();
  const [contact, setContact] = usePersistedState(`agents.detail.${agentId}.wechat-contact`, "");
  const [query, setQuery] = usePersistedState(`agents.detail.${agentId}.wechat-query`, "");
  const [senderValue, setSenderValue] = usePersistedState(`agents.detail.${agentId}.wechat-sender`, "all");
  const [sortValue, setSortValue] = usePersistedState(`agents.detail.${agentId}.wechat-sort`, "desc");
  const [viewValue, setViewValue] = usePersistedState(`agents.detail.${agentId}.wechat-view`, "chat");
  const sender: WeChatSenderFilter = senderValue === "me" || senderValue === "other" ? senderValue : "all";
  const sort: WeChatSortDirection = sortValue === "asc" ? "asc" : "desc";
  const view: "chat" | "table" = viewValue === "table" ? "table" : "chat";
  const { busy, collect } = useCollectTask(agentId);
  const collecting = busy !== null;
  const [raw, setRaw] = useState("");
  const [collected, setCollected] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);

  useEffect(() => {
    setRaw("");
    setCollected(false);
    setSelected(null);
  }, [agentId]);

  const meta = useMemo(() => parseWeChatHistoryWithMeta(raw), [raw]);
  const filtered = useMemo(() => filterWeChatMessages(meta.messages, query, sender), [meta.messages, query, sender]);
  const sorted = useMemo(() => sortWeChatMessages(filtered, sort), [filtered, sort]);
  const conversations = useMemo(() => groupWeChatConversations(filtered), [filtered]);
  const active = selected ? conversations.find((conversation) => conversation.id === selected) ?? null : null;
  const conversationMessages = useMemo(() => {
    if (!active) return sorted;
    return sorted.filter((message) => (message.contactId || message.contact) === active.id);
  }, [sorted, active]);
  const rendered = conversationMessages.slice(0, 500);

  const handleCollect = async () => {
    const output = await collect("wechat", paths.agents.wechatHistory(agentId), {
      body: { contact: contact.trim() },
      storeResult: false,
      errorText: t("agents.wechat_collect_failed"),
    });
    if (output === null) return;
    setRaw(output);
    setCollected(true);
    setSelected(null);
    const parsed = parseWeChatHistoryWithMeta(output);
    toast.success(t("agents.wechat_collected").replace("{n}", String(parsed.messages.length)));
  };

  const handleExport = (format: "json" | "csv") => {
    if (conversationMessages.length === 0) return;
    try {
      const date = new Date().toISOString().slice(0, 10);
      const stem = `wechat-${agentId}-${date}`;
      if (format === "json") {
        downloadJSON(conversationMessages, `${stem}.json`);
      } else {
        downloadText(toWeChatCSV(conversationMessages), `${stem}.csv`, "text/csv");
      }
      toast.success(t("agents.wechat_exported").replace("{n}", String(conversationMessages.length)));
    } catch {
      toast.error(t("agents.wechat_export_failed"));
    }
  };

  return (
    <Card className="mb-4 overflow-hidden">
      <div className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-3">
        <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
          <MessageSquare className="size-3.5 text-primary" />
          {t("agents.wechat_title")}
        </h3>
        <span className="ml-auto flex items-center gap-2">
          <Input
            value={contact}
            onChange={(e) => setContact(e.target.value)}
            placeholder={t("agents.wechat_contact_placeholder")}
            aria-label={t("agents.wechat_contact_placeholder")}
            className="h-8 w-44 text-xs"
          />
          <Button size="sm" onClick={() => void handleCollect()} disabled={!online || collecting}>
            {collecting ? (
              <>
                <Spinner size="xs" /> {t("agents.wechat_collecting")}
              </>
            ) : (
              <>
                <MessageSquare className="size-4" /> {t("agents.wechat_collect")}
              </>
            )}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => handleExport("json")}
            disabled={!online || collecting || conversationMessages.length === 0}
          >
            <Download className="size-4" /> {t("agents.wechat_export_json")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => handleExport("csv")}
            disabled={!online || collecting || conversationMessages.length === 0}
          >
            <Download className="size-4" /> {t("agents.wechat_export_csv")}
          </Button>
        </span>
      </div>
      <div className="p-3">
        {!collected && !collecting && (
          <EmptyState
            icon={MessageSquare}
            title={t("agents.wechat_empty")}
            message={t("agents.wechat_empty_hint")}
          />
        )}
        {collecting && !collected && (
          <div className="flex items-center justify-center gap-3 py-8">
            <Spinner size="sm" />
            <p className="text-sm text-muted-foreground">{t("agents.wechat_collecting")}</p>
          </div>
        )}
        {collected && (
          <>
            <div className="mb-2 grid gap-2 lg:grid-cols-[1fr_auto]">
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={t("agents.wechat_search_placeholder")}
                  aria-label={t("agents.wechat_search_placeholder")}
                  className="h-8 pl-8 text-xs"
                />
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Select value={sender} onValueChange={(v) => v !== null && setSenderValue(v)}>
                  <SelectTrigger className="h-8 w-28 text-xs">
                    <SelectValue placeholder={t("agents.wechat_sender_all")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t("agents.wechat_sender_all")}</SelectItem>
                    <SelectItem value="me">{t("agents.wechat_sender_me")}</SelectItem>
                    <SelectItem value="other">{t("agents.wechat_sender_other")}</SelectItem>
                  </SelectContent>
                </Select>
                <Button size="sm" variant="outline" onClick={() => setSortValue(sort === "desc" ? "asc" : "desc")}>
                  {sort === "desc" ? t("agents.wechat_sort_newest") : t("agents.wechat_sort_oldest")}
                </Button>
                <Button size="sm" variant={view === "chat" ? "default" : "outline"} onClick={() => setViewValue("chat")}>
                  {t("agents.wechat_view_chat")}
                </Button>
                <Button size="sm" variant={view === "table" ? "default" : "outline"} onClick={() => setViewValue("table")}>
                  {t("agents.wechat_view_table")}
                </Button>
              </div>
            </div>
            {meta.skipped > 0 && (
              <p className="mb-2 rounded-lg border border-warning/40 bg-warning/10 px-3 py-2 text-xs text-warning-foreground">
                {t("agents.wechat_skipped_warning").replace("{n}", String(meta.skipped))}
              </p>
            )}
            {meta.queryErrors.length > 0 && (
              <div className="mb-2 rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                <p className="font-medium text-foreground">{t("agents.wechat_query_errors")}</p>
                <ul className="mt-1 list-disc space-y-1 pl-4">
                  {meta.queryErrors.slice(0, 3).map((error, i) => (
                    <li key={`${error}-${i}`} className="break-words font-mono">
                      {error}
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {rendered.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">
                {meta.hasNoSource ? t("agents.wechat_no_source") : t("agents.wechat_no_match")}
              </p>
            ) : view === "chat" ? (
              <div className="grid gap-3 lg:grid-cols-[230px_1fr]">
                <div className="max-h-96 space-y-1 overflow-auto rounded-lg border border-border p-2">
                  <Button
                    size="sm"
                    variant={active ? "outline" : "default"}
                    className="w-full justify-between"
                    onClick={() => setSelected(null)}
                  >
                    <span>{t("agents.wechat_all_conversations")}</span>
                    <span className="font-mono text-xs">{filtered.length}</span>
                  </Button>
                  {conversations.map((conversation) => (
                    <Button
                      key={conversation.id}
                      size="sm"
                      variant={active?.id === conversation.id ? "default" : "outline"}
                      className="h-auto w-full flex-col items-start gap-0.5 py-2"
                      onClick={() => setSelected(conversation.id)}
                    >
                      <span className="block w-full truncate text-left text-xs font-medium">
                        {conversation.contact}
                      </span>
                      <span className="block w-full truncate text-left font-mono text-xs opacity-70" title={conversation.contactId}>
                        {conversation.contactId} · {conversation.count}
                      </span>
                      <span className="block w-full truncate text-left font-mono text-xs opacity-70">
                        {conversation.latest}
                      </span>
                    </Button>
                  ))}
                </div>
                <div className="max-h-96 space-y-2 overflow-auto rounded-lg border border-border p-3">
                  {rendered.map((r, i) => {
                    const isMe = r.sender.toLowerCase() === "me";
                    return (
                      <div key={`${r.contactId}-${r.time}-${i}`} className={`flex ${isMe ? "justify-end" : "justify-start"}`}>
                        <div className="max-w-[90%] rounded-lg border border-border bg-muted/40 p-2.5">
                          <div className="mb-1 flex items-center justify-between gap-3">
                            <span className="truncate font-mono text-xs text-muted-foreground">
                              {r.time} · {isMe ? t("agents.wechat_sender_me") : t("agents.wechat_sender_other")}
                            </span>
                            <CopyButton text={r.content} label={t("agents.wechat_copy_message")} size="xs" />
                          </div>
                          <p className="mb-1 truncate text-xs text-muted-foreground" title={r.contactId}>
                            {r.contact}
                          </p>
                          <p className="whitespace-pre-wrap break-words text-xs text-foreground">{r.content || "—"}</p>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            ) : (
              <div className="max-h-96 overflow-auto rounded-lg border border-border">
                <Table>
                  <TableHeader>
                    <TableRow className="bg-muted/50">
                      <TableHead className="w-40">{t("agents.wechat_col_time")}</TableHead>
                      <TableHead className="w-20">{t("agents.wechat_col_sender")}</TableHead>
                      <TableHead className="w-48">{t("agents.wechat_col_contact")}</TableHead>
                      <TableHead className="w-16">{t("agents.wechat_col_type")}</TableHead>
                      <TableHead>{t("agents.wechat_col_content")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {rendered.map((r, i) => (
                      <TableRow key={`${r.contactId}-${r.time}-${i}`}>
                        <TableCell className="font-mono text-xs text-muted-foreground">{r.time || "—"}</TableCell>
                        <TableCell className="text-xs text-muted-foreground">{r.sender || "—"}</TableCell>
                        <TableCell className="min-w-0">
                          <span className="block max-w-48 truncate text-xs text-foreground" title={r.contactId}>
                            {r.contact || "—"}
                          </span>
                          {r.contactId && (
                            <span className="block max-w-48 truncate font-mono text-xs text-muted-foreground" title={r.contactId}>
                              {r.contactId}
                            </span>
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">{r.kind || "—"}</TableCell>
                        <TableCell className="min-w-0">
                          <span className="block max-w-2xl whitespace-pre-wrap break-words text-xs text-foreground">
                            {r.content || "—"}
                          </span>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
            {conversationMessages.length > 500 && (
              <p className="mt-1 text-xs text-muted-foreground">
                {t("agents.wechat_truncated").replace("{n}", String(conversationMessages.length - 500))}
              </p>
            )}
          </>
        )}
      </div>
    </Card>
  );
});
