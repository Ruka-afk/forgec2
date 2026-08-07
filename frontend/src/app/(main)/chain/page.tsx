"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";

import { PageHeader, PageSpinner } from "@/components/UI";
import { useAgentList } from "@/lib/hooks/useAgentList";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";
import { ArrowRight, Bug, Link, Pencil, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";

interface ChainNode {
  id: string;
  hostname: string;
  parent_id: string;
}

export default function ChainPage() {
  const { t } = useI18n();

  const { agents } = useAgentList();
  const [graph, setGraph] = useState<ChainNode[]>([]);
  const [selectedAgent, setSelectedAgent] = useState<string>("");
  const [chain, setChain] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [actionMsg, setActionMsg] = useState<string | null>(null);

  const fetchGraph = useCallback(async () => {
    try {
      const data = await api.get<{ nodes: ChainNode[] }>("/chain/graph");
      setGraph(data.nodes || []);
    } catch {
      toast.error(t("chain.load_failed"));
    }
  }, [t]);

  const loadChain = useCallback(async (agentId: string) => {
    if (!agentId) {
      setChain([]);
      return;
    }
    try {
      const data = await api.get<{ chain: string[] }>(`/agents/${agentId}/chain`);
      setChain(data.chain || []);
    } catch {
      setChain([]);
      toast.error(t("chain.load_proxy_failed"));
    }
  }, [t]);

  useEffect(() => {
    fetchGraph().finally(() => setLoading(false));
  }, [fetchGraph]);

  useEffect(() => {
    if (selectedAgent) {
      loadChain(selectedAgent);
    } else {
      setChain([]);
    }
  }, [selectedAgent, loadChain]);

  const getAgentInfo = (id: string) => {
    return agents.find((a) => a.id === id);
  };

  const handleSetParent = async (parentId: string) => {
    if (!selectedAgent) return;
    try {
      await api.postJson(`/agents/${selectedAgent}/chain/set`, {
        parent_agent_id: parentId,
      });
      setActionMsg(t("chain.parent_set", { id: parentId }));
      setShowModal(false);
      loadChain(selectedAgent);
      fetchGraph();
    } catch {
      setActionMsg(t("chain.set_failed"));
    }
  };

  const handleClearChain = async () => {
    if (!selectedAgent) return;
    try {
      await api.postJson(`/agents/${selectedAgent}/chain/clear`, {});
      setActionMsg(t("chain.cleared"));
      loadChain(selectedAgent);
      fetchGraph();
    } catch {
      setActionMsg(t("chain.clear_failed"));
    }
  };

  const selectedInfo = getAgentInfo(selectedAgent);
  const availableParents = agents.filter((a) => a.id !== selectedAgent);

  if (loading) {
    return <PageSpinner />;
  }

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader
        title={<span><Link className="w-4 h-4" />{t("chain.multi_hop")}</span>}
        subtitle={t("chain.subtitle")}
      />

      {/* Agent Selector */}
      <Card className="p-4 mb-6">
        <Label className="text-xs font-medium text-muted-foreground mb-2 block">{t("chain.select_agent")}</Label>
        <Select value={selectedAgent} onValueChange={(v) => setSelectedAgent(v ?? "")}>
          <SelectTrigger className="w-full">
            <SelectValue placeholder={t("chain.select_agent_placeholder")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">{t("chain.select_agent_placeholder")}</SelectItem>
            {agents.map((a) => (
              <SelectItem key={a.id || ""} value={a.id || ""}>
                {a.hostname} ({a.ip}) — {(a.id || "").substring(0, 8)}...
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Card>

      {selectedAgent && (
        <>
          {/* Current Configuration */}
          <Card className="p-4 mb-6">
            <h3 className="text-sm font-semibold text-foreground mb-3">{t("chain.current_chain")}</h3>
            {chain.length > 0 ? (
              <div className="flex flex-wrap items-center gap-2 text-sm">
                {chain.map((id, idx) => {
                  const info = getAgentInfo(id);
                  return (
                    <span key={id} className="flex items-center gap-1">
                      <Badge variant="secondary" className="font-mono text-xs">
                        {info ? `${info.hostname}` : id.substring(0, 8)}
                      </Badge>
                      {idx < chain.length - 1 && (
                        <ArrowRight className="w-4 h-4" />
                      )}
                    </span>
                  );
                })}
                <ArrowRight className="w-4 h-4" />
                <Badge variant="success" className="text-xs font-medium">
                  C2
                </Badge>
              </div>
            ) : (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Badge variant="secondary" className="font-mono text-xs">
                  {selectedInfo ? selectedInfo.hostname : selectedAgent.substring(0, 8)}
                </Badge>
                <ArrowRight className="w-4 h-4" />
                <Badge variant="success" className="text-xs font-medium">
                  C2 (Direct)
                </Badge>
              </div>
            )}
          </Card>

          {/* Actions */}
          <div className="flex flex-wrap gap-3 mb-6">
            <Button
              size="sm"
              onClick={() => setShowModal(true)}
            >
              <Pencil className="w-4 h-4" /> {t("chain.set_parent")}
            </Button>
            <Button
              size="sm"
              variant="destructive"
              onClick={handleClearChain}
            >
              <X className="w-4 h-4" /> {t("chain.clear_chain")}
            </Button>
          </div>

          {/* Parent Selection Dialog */}
          <Dialog open={showModal} onOpenChange={setShowModal}>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>{t("chain.set_proxy_parent")}</DialogTitle>
              </DialogHeader>
              <p className="text-sm text-muted-foreground">
                {t("chain.select_parent_desc", { name: selectedInfo?.hostname || selectedAgent.substring(0, 8) })}
              </p>
              <div className="max-h-60 overflow-y-auto space-y-1">
                {availableParents.length === 0 ? (
                  <p className="text-xs text-muted-foreground/70 py-4 text-center">{t("chain.no_other_agents")}</p>
                ) : (
                  availableParents.map((a) => (
                    <Button
                      variant="ghost"
                      size="sm"
                      key={a.id || ""}
                      onClick={() => a.id && handleSetParent(a.id)}
                      className="w-full justify-start text-left h-auto px-3 py-2"
                    >
                      <Bug className="w-4 h-4" />
                      <span className="font-medium text-foreground">{a.hostname}</span>
                      <span className="text-muted-foreground/70 ml-1">({a.ip})</span>
                      <span className="text-(--fs-micro-sm) text-muted-foreground/70 font-mono ml-auto">{(a.id || "").substring(0, 8)}...</span>
                    </Button>
                  ))
                )}
              </div>
              <DialogFooter>
                <Button variant="outline" size="sm" onClick={() => setShowModal(false)}>
                  {t("common.cancel")}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* Chain Visualization - Full Topology */}
          {graph.length > 0 && (
            <Card className="p-4 mb-6">
              <h3 className="text-sm font-semibold text-foreground mb-3">{t("chain.topology_view")}</h3>
              <div className="space-y-2">
                {graph
                  .filter((n) => n.parent_id || n.id === selectedAgent)
                  .map((node) => {
                    const info = getAgentInfo(node.id);
                    return (
                      <div key={node.id} className="flex items-center gap-2">
                        <div className={`px-3 py-1.5 rounded-lg text-xs font-mono ${
                          node.id === selectedAgent
                            ? "bg-primary/10 text-primary ring-2 ring-primary/60"
                            : node.parent_id === ""
                            ? "bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300"
                            : "bg-muted text-foreground"
                        }`}>
                          {info ? `${info.hostname} (${info.ip})` : node.id.substring(0, 12)}
                        </div>
                        {node.parent_id ? (
                          <>
                            <ArrowRight className="w-4 h-4" />
                            <span className="text-xs text-muted-foreground/70">
                              via {getAgentInfo(node.parent_id)?.hostname || node.parent_id.substring(0, 8)}
                            </span>
                          </>
                        ) : (
                          <>
                            <ArrowRight className="w-4 h-4" />
                            <Badge variant="success" className="text-(--fs-micro-sm) font-medium">
                              C2
                            </Badge>
                          </>
                        )}
                      </div>
                    );
                  })}
              </div>
            </Card>
          )}
        </>
      )}

      {/* Action Message */}
      {actionMsg && (
        <Card className="p-4 mb-6 flex items-center justify-between">
          <span className="text-sm text-foreground">{actionMsg}</span>
          <Button variant="ghost" size="sm" onClick={() => setActionMsg(null)}>
            <X className="w-4 h-4" />
          </Button>
        </Card>
      )}
    </div>
  );
}
