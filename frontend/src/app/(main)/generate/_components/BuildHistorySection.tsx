"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { formatTime } from "@/lib/utils";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CheckCircle, ChevronDown, History, XCircle } from "lucide-react";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { BuildHistoryEntry } from "@/types/generate";

const FORMAT_ICONS: Record<string, string> = {
  exe: "\u{1F5A5}\uFE0F", dll: "\u{1F9E9}", ps1: "\u{1F4DC}", linux: "\u{1F427}", macos: "\u{1F34F}",
  stager: "📦", shellcode: "💻", donut: "🍩",
};

export default function BuildHistorySection({ refreshKey }: { refreshKey?: number }) {
  const { t } = useI18n();
  const [builds, setBuilds] = useState<BuildHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState(false);
  useEffect(() => {
    api.get<{ builds?: BuildHistoryEntry[]; Logs?: BuildHistoryEntry[] }>("/builds?pageSize=20")
      .then((d) => setBuilds(d.builds || d.Logs || []))
      .catch(() => { toast.error(t("generate.toast.build_history_load_failed")); })
      .finally(() => setLoading(false));
  }, [refreshKey, t]);

  if (loading || builds.length === 0) return null;

  return (
    <div className="mt-8">
      <Collapsible open={expanded} onOpenChange={setExpanded}>
        <CollapsibleTrigger>
          <Button type="button" variant="ghost" className="flex items-center gap-x-3 mb-5 w-full text-left justify-start h-auto py-0">
            <div className="w-10 h-10 bg-card rounded-xl flex items-center justify-center"><History className="w-4 h-4" /></div>
            <div className="flex-1">
              <div className="text-sm font-semibold text-foreground">Build History</div>
              <div className="text-xs text-muted-foreground">{builds.length} recent builds</div>
            </div>
            <ChevronDown className="w-3 h-3 text-muted-foreground" />
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <Card className="p-4 overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow className="text-xs text-muted-foreground">
                  <TableHead className="text-left font-medium">Time</TableHead>
                  <TableHead className="text-left font-medium">Format</TableHead>
                  <TableHead className="text-left font-medium">Filename</TableHead>
                  <TableHead className="text-left font-medium">Status</TableHead>
                  <TableHead className="text-left font-medium">User</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {builds.slice(0, 20).map((b) => (
                  <TableRow key={b.id}>
                    <TableCell className="text-xs text-muted-foreground whitespace-nowrap">{formatTime(b.created_at)}</TableCell>
                    <TableCell><span className="text-xs">{FORMAT_ICONS[b.format] || "📄"}</span> <span className="text-xs font-medium text-muted-foreground uppercase">{b.format}</span></TableCell>
                    <TableCell className="text-xs font-mono text-muted-foreground">{b.filename || "\u2014"}</TableCell>
                    <TableCell>
                      {b.status === "success" ? (
                        <Badge variant="success" className="text-xs gap-1"><CheckCircle className="w-4 h-4" /> OK</Badge>
                      ) : (
                        <Tooltip>
                          <TooltipTrigger>
                            <Badge variant="destructive" className="text-xs gap-1"><XCircle className="w-4 h-4" /> Failed</Badge>
                          </TooltipTrigger>
                          <TooltipContent>{b.error}</TooltipContent>
                        </Tooltip>
                      )}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">{b.user || "system"}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}
