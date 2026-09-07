import { useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";

export interface InfraListener {
  id: string;
  name: string;
  host: string;
  port: string;
  protocol: string;
}

interface Redirector {
  id: number;
  name: string;
  host: string;
  type: string;
  status: string;
  config: string;
  ssh_user: string;
  ssh_port: number;
  last_seen: string;
  created_at: string;
  updated_at: string;
}

export function useInfrastructureData() {
  const { t } = useI18n();
  const [listeners, setListeners] = useState<InfraListener[]>([]);
  const [redirectors, setRedirectors] = useState<Redirector[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadListeners = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<{ Listeners?: InfraListener[]; listeners?: InfraListener[] }>(paths.listeners.list);
      setListeners((data.listeners || []) as InfraListener[]);
    } catch {
      setListeners([]);
      setError(t("infrastructure.toast.load_listeners_failed"));
      toast.error(t("infrastructure.toast.load_listeners_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  const loadRedirectors = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<{ redirectors?: Redirector[] }>("/redirectors");
      setRedirectors((data.redirectors || []) as Redirector[]);
    } catch {
      setRedirectors([]);
      setError(t("infrastructure.toast.load_redirectors_failed"));
      toast.error(t("infrastructure.toast.load_redirectors_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  return { listeners, redirectors, loading, error, loadListeners, loadRedirectors };
}

