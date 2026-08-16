"use client";

import { memo } from "react";
import Link from "next/link";
import { Card } from "@/components/ui/card";
import { StatusBadge } from "@/components/ui/status-indicator";
import { GitBranch } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { getOSIcon, type AgentDetailModel } from "./agent-detail-utils";

interface AgentChildListProps {
  childAgents: AgentDetailModel[];
}

export default memo(function AgentChildList({ childAgents }: AgentChildListProps) {
  const { t } = useI18n();
  if (childAgents.length === 0) return null;

  return (
    <Card className="p-5 mb-5">
      <div className="text-(--fs-xs-sm) font-semibold uppercase tracking-wider text-muted-foreground/70 mb-3.5">
        <GitBranch className="w-3.5 h-3.5 inline mr-1" aria-hidden="true" />
        {t("agents.detail_child_agents")} ({childAgents.length})
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
        {childAgents.map((child) => {
          const cid = child.id || "";
          const ch = child.hostname || "—";
          const cos = child.os || "";
          const cip = child.ip || "";
          const cs = child.status || "offline";
          const cp = child.p2p_mode || "";
          const OSIcon = getOSIcon(cos);
          return (
            <Link
              key={cid}
              href={`/agents/${cid}`}
              className="flex items-center gap-3 p-2.5 rounded-lg bg-muted/50 hover:bg-muted transition-colors group"
            >
              <div className="w-8 h-8 rounded-lg bg-primary/10 dark:bg-primary/20 flex items-center justify-center shrink-0">
                <OSIcon className="w-4 h-4 text-primary" aria-hidden="true" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-xs font-medium text-foreground truncate group-hover:text-primary dark:group-hover:text-primary transition-colors">
                  {ch}
                </div>
                <div className="text-(--fs-micro-sm) text-muted-foreground/70">
                  {cip}{cp ? ` (${cp})` : ""}
                </div>
              </div>
              <StatusBadge status={cs} />
            </Link>
          );
        })}
      </div>
    </Card>
  );
})
