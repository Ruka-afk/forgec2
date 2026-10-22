import { memo } from "react";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Download,
  Grip,
  History,
  Layers,
  ListChecks,
  ListOrdered,
  MoreHorizontal,
  Pause,
  Play,
  Plus,
  RefreshCw,
} from "lucide-react";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface AgentsToolbarProps {
  t: TKey;
  bulkMode: boolean;
  onToggleBulk: () => void;
  showResults: boolean;
  onToggleResults: () => void;
  exporting: boolean;
  onExport: () => void;
  autoRefresh: boolean;
  onToggleAutoRefresh: () => void;
  effectiveViewMode: "table" | "grid";
  onToggleView: () => void;
  groupByHost: boolean;
  onToggleGroupByHost: () => void;
  onRefresh: () => void;
}

/**
 * Page actions for the agents list. One primary action plus an overflow menu:
 * the header used to render eight same-weight buttons, which buried the
 * Generate Payload entry point.
 */
export default memo(function AgentsToolbar({
  t, bulkMode, onToggleBulk, showResults, onToggleResults,
  exporting, onExport, autoRefresh, onToggleAutoRefresh,
  effectiveViewMode, onToggleView, groupByHost, onToggleGroupByHost, onRefresh,
}: AgentsToolbarProps) {
  return (
    <>
      <Button
        variant="ghost"
        size="icon"
        onClick={onRefresh}
        aria-label={t("agents.refresh")}
        title={t("agents.refresh")}
      >
        <RefreshCw className="size-4" />
      </Button>
      <Button
        variant={autoRefresh ? "secondary" : "ghost"}
        size="icon"
        onClick={onToggleAutoRefresh}
        aria-pressed={autoRefresh}
        aria-label={autoRefresh ? t("agents.auto_refresh_on") : t("agents.auto_refresh_off")}
        title={autoRefresh ? t("agents.auto_refresh_on") : t("agents.auto_refresh_off")}
      >
        {autoRefresh ? <Pause className="size-4" /> : <Play className="size-4" />}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button variant="outline" aria-label={t("common.more")}>
              <MoreHorizontal className="size-4" />
              <span className="hidden sm:inline">{t("common.more")}</span>
            </Button>
          }
        />
        <DropdownMenuContent className="w-56">
          <DropdownMenuCheckboxItem checked={bulkMode} onCheckedChange={() => onToggleBulk()}>
            <ListChecks className="size-4" /> {t("agents.bulk_ops")}
          </DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem checked={showResults} onCheckedChange={() => onToggleResults()}>
            <History className="size-4" /> {t("agents.results")}
          </DropdownMenuCheckboxItem>
          <DropdownMenuItem disabled={exporting} onClick={onExport}>
            <Download className="size-4" /> {t("agents.export")}
            <span className="ml-auto text-micro text-muted-foreground">CSV</span>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={onToggleView}>
            {effectiveViewMode === "table" ? <Grip className="size-4" /> : <ListOrdered className="size-4" />}
            {effectiveViewMode === "table" ? t("agents.switch_grid") : t("agents.switch_table")}
          </DropdownMenuItem>
          {effectiveViewMode === "table" && (
            <DropdownMenuCheckboxItem checked={groupByHost} onCheckedChange={() => onToggleGroupByHost()}>
              <Layers className="size-4" /> {t("agents.group_by_host")}
            </DropdownMenuCheckboxItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <Button render={<Link to="/payload-generator" />}>
        <Plus className="size-4" />
        <span className="hidden sm:inline">{t("agents.generate_implant")}</span>
        <span className="sm:hidden">{t("agents.new")}</span>
      </Button>
    </>
  );
});
