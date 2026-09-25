
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import type { Campaign, CampaignStats } from "./types";

export function useCampaignData() {
  const { t } = useI18n();
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState<string | null>(null);
  const [selectedCampaign, setSelectedCampaign] = useState<string | null>(null);
  const [campaignStats, setCampaignStats] = useState<CampaignStats | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const loadCampaigns = useCallback(
    async (signal?: AbortSignal) => {
      setListError(null);
      try {
        // api.get auto-unwraps {success,data} — the value IS the array.
        const data = await api.get<Campaign[] | { success: boolean; data?: Campaign[] }>(paths.campaigns.list, {
          signal,
        });
        if (Array.isArray(data)) {
          setCampaigns(data);
        } else if (Array.isArray((data as { data?: Campaign[] }).data)) {
          setCampaigns((data as { data: Campaign[] }).data);
        } else {
          setCampaigns([]);
        }
      } catch (e) {
        if (!signal?.aborted) {
          // Keep the list empty but say why: an unread list must not render
          // as "you have no campaigns".
          const msg = e instanceof Error ? e.message : t("campaign.toast.load_failed");
          setListError(msg);
          toast.error(msg);
        }
      } finally {
        if (!signal?.aborted) setLoading(false);
      }
    },
    [t],
  );

  const loadCampaignDetail = useCallback(
    async (id: string, signal?: AbortSignal) => {
      // campaignStats === null used to mean both "loading" and "failed", so a
      // failed detail fetch left the detail panel spinning forever.
      setStatsLoading(true);
      setStatsError(null);
      try {
        const data = await api.get<{ stats?: CampaignStats }>(paths.campaigns.one(id), { signal });
        if (signal?.aborted) return;
        setCampaignStats(data.stats || null);
      } catch {
        if (signal?.aborted) return;
        setStatsError(t("campaign.toast.load_stats_failed"));
        toast.error(t("campaign.toast.load_stats_failed"));
      } finally {
        if (!signal?.aborted) setStatsLoading(false);
      }
    },
    [t],
  );

  useEffect(() => {
    const controller = new AbortController();
    void loadCampaigns(controller.signal);
    return () => controller.abort();
  }, [loadCampaigns]);

  useEffect(() => {
    const controller = new AbortController();
    if (selectedCampaign) {
      // Clear immediately so switching campaigns never shows the previous
      // campaign's stats under the new selection while loading.
      setCampaignStats(null);
      void loadCampaignDetail(selectedCampaign, controller.signal);
    } else {
      setCampaignStats(null);
    }
    return () => controller.abort();
  }, [selectedCampaign, loadCampaignDetail]);

  const createCampaign = useCallback(
    async (name: string, description: string) => {
      if (!name.trim()) return false;
      setCreating(true);
      try {
        await api.postJson(paths.campaigns.list, { name, description });
        void loadCampaigns();
        toast.success(t("campaign.toast.created"));
        return true;
      } catch {
        toast.error(t("campaign.toast.load_failed"));
        return false;
      } finally {
        setCreating(false);
      }
    },
    [loadCampaigns, t],
  );

  const deleteCampaign = useCallback(
    async (id: string) => {
      try {
        await api.del(paths.campaigns.one(id));
        if (selectedCampaign === id) setSelectedCampaign(null);
        void loadCampaigns();
        toast.success(t("campaign.toast.deleted"));
      } catch {
        toast.error(t("campaign.toast.load_failed"));
      }
    },
    [selectedCampaign, loadCampaigns, t],
  );

  const updateStatus = useCallback(
    async (id: string, status: string) => {
      try {
        await api.postJson(paths.campaigns.one(id), { status });
        void loadCampaigns();
        if (selectedCampaign === id) void loadCampaignDetail(id);
        toast.success(t("campaign.toast.status_updated"));
      } catch {
        toast.error(t("campaign.toast.status_update_failed"));
      }
    },
    [selectedCampaign, loadCampaigns, loadCampaignDetail, t],
  );

  return {
    campaigns,
    loading,
    listError,
    selectedCampaign,
    setSelectedCampaign,
    campaignStats,
    statsLoading,
    statsError,
    creating,
    loadCampaigns,
    loadCampaignDetail,
    createCampaign,
    deleteCampaign,
    updateStatus,
  };
}
