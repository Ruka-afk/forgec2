"use client";

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
  const [selectedCampaign, setSelectedCampaign] = useState<string | null>(null);
  const [campaignStats, setCampaignStats] = useState<CampaignStats | null>(null);
  const [creating, setCreating] = useState(false);

  const loadCampaigns = useCallback(
    async (signal?: AbortSignal) => {
      try {
        const data = await api.get<{ success: boolean; data?: Campaign[] }>(paths.campaigns.list, {
          signal,
        });
        if (data.success) setCampaigns(data.data || []);
        else if (Array.isArray((data as { data?: Campaign[] }).data)) {
          setCampaigns((data as { data: Campaign[] }).data);
        }
      } catch {
        if (!signal?.aborted) toast.error(t("campaign.toast.load_failed"));
      } finally {
        if (!signal?.aborted) setLoading(false);
      }
    },
    [t],
  );

  const loadCampaignDetail = useCallback(
    async (id: string, signal?: AbortSignal) => {
      try {
        const data = await api.get<{ stats?: CampaignStats }>(paths.campaigns.one(id), { signal });
        setCampaignStats(data.stats || null);
      } catch {
        if (!signal?.aborted) toast.error(t("campaign.toast.load_stats_failed"));
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
    if (selectedCampaign) void loadCampaignDetail(selectedCampaign, controller.signal);
    else setCampaignStats(null);
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
        toast.error(t("campaign.toast.status_updated"));
      }
    },
    [selectedCampaign, loadCampaigns, loadCampaignDetail, t],
  );

  return {
    campaigns,
    loading,
    selectedCampaign,
    setSelectedCampaign,
    campaignStats,
    creating,
    loadCampaigns,
    loadCampaignDetail,
    createCampaign,
    deleteCampaign,
    updateStatus,
  };
}
