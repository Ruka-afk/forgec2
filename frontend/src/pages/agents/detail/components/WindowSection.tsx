import { memo, useMemo, useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { toast } from "sonner";
import { AppWindow, ListChecks, Search, XCircle } from "lucide-react";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { parseWindowList } from "./window-list";
import { useCollectTask } from "./useCollectTask";

interface WindowSectionProps {
  agentId: string;
  online: boolean;
}

export default memo(function WindowSection({ agentId, online }: WindowSectionProps) {
  const { t } = useI18n();
  const [target, setTarget] = useState("");
  const { busy, collect } = useCollectTask(agentId);
  const collecting = busy !== null;
  const [raw, setRaw] = useState("");
  const [collected, setCollected] = useState(false);
  const [query, setQuery] = useState("");
  // Cap first paint: 500 table rows mount at once without virtualization.
  const [renderLimit, setRenderLimit] = useState(100);

  const rows = useMemo(() => parseWindowList(raw), [raw]);
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter(
      (r) =>
        r.hwnd.includes(q) ||
        r.pid.includes(q) ||
        r.title.toLowerCase().includes(q),
    );
  }, [rows, query]);

  const handleList = async () => {
    const output = await collect("list", paths.agents.windowList(agentId), {
      storeResult: false,
      errorText: t("agents.window_failed"),
    });
    if (output === null) return;
    setRaw(output);
    setCollected(true);
    const n = parseWindowList(output).length;
    toast.success(t("agents.window_listed").replace("{n}", String(n)));
  };

  const handleClose = async (hwndOrTitle: string) => {
    const output = await collect("close", paths.agents.windowClose(agentId), {
      body: { command: hwndOrTitle },
      storeResult: false,
      successText: t("agents.window_closed"),
      errorText: t("agents.window_failed"),
    });
    if (output === null) return;
    // Refresh the list so a closed window disappears without manual reload.
    await handleList();
  };

  return (
    <Card className="mb-4 overflow-hidden">
      <div className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-3">
        <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
          <AppWindow className="size-3.5 text-primary" />
          {t("agents.window_title")}
        </h3>
        <span className="ml-auto flex items-center gap-2">
          <Input
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            placeholder={t("agents.window_close_target_ph")}
            aria-label={t("agents.window_close_target_ph")}
            className="h-8 w-44 font-mono text-xs"
          />
          <Button
            size="sm"
            variant="outline"
            onClick={() => void handleClose(target.trim())}
            disabled={!online || collecting || !target.trim()}
          >
            {busy === "close" ? (
              <>
                <Spinner size="xs" /> {t("agents.window_closing")}
              </>
            ) : (
              <>
                <XCircle className="size-4" /> {t("agents.window_close")}
              </>
            )}
          </Button>
          <Button size="sm" onClick={() => void handleList()} disabled={!online || collecting}>
            {busy === "list" ? (
              <>
                <Spinner size="xs" /> {t("agents.window_collecting")}
              </>
            ) : (
              <>
                <ListChecks className="size-4" /> {t("agents.window_list")}
              </>
            )}
          </Button>
        </span>
      </div>
      <div className="p-3">
        {!collected && !collecting && (
          <EmptyState
            icon={AppWindow}
            title={t("agents.window_empty")}
            message={t("agents.window_empty_hint")}
          />
        )}
        {collecting && !collected && (
          <div className="flex items-center justify-center gap-3 py-8">
            <Spinner size="sm" />
            <p className="text-sm text-muted-foreground">{t("agents.window_collecting")}</p>
          </div>
        )}
        {collected && (
          <>
            <div className="relative mb-2">
              <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("agents.window_search_placeholder")}
                aria-label={t("agents.window_search_placeholder")}
                className="h-8 pl-8 text-xs"
              />
            </div>
            {visible.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">{t("agents.window_no_match")}</p>
            ) : (
              <div className="max-h-96 overflow-auto rounded-lg border border-border">
                <Table>
                  <TableHeader>
                    <TableRow className="bg-muted/50">
                      <TableHead className="w-32">{t("agents.window_col_hwnd")}</TableHead>
                      <TableHead className="w-24">{t("agents.window_col_pid")}</TableHead>
                      <TableHead>{t("agents.window_col_title")}</TableHead>
                      <TableHead className="w-24" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {visible.slice(0, renderLimit).map((r) => (
                      <TableRow key={r.hwnd}>
                        <TableCell className="font-mono text-xs text-muted-foreground">{r.hwnd}</TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">{r.pid}</TableCell>
                        <TableCell className="min-w-0">
                          <span className="block max-w-xl truncate text-xs text-foreground" title={r.title}>
                            {r.title || "—"}
                          </span>
                        </TableCell>
                        <TableCell>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => void handleClose(r.hwnd)}
                            disabled={!online || collecting}
                          >
                            {t("agents.window_close")}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
            {visible.length > renderLimit && (
              <div className="mt-2 flex items-center justify-center">
                <Button size="sm" variant="outline" onClick={() => setRenderLimit((n) => n + 100)}>
                  {t("agents.tasklist_load_more").replace("{count}", String(Math.min(100, visible.length - renderLimit)))}
                </Button>
              </div>
            )}
            {visible.length > 500 && (
              <p className="mt-1 text-xs text-muted-foreground">
                {t("agents.window_truncated").replace("{n}", String(visible.length - 500))}
              </p>
            )}
          </>
        )}
      </div>
    </Card>
  );
});
