import { useState, useCallback } from "react";
import { api } from "@/lib/api";
import { toast } from "sonner";

export interface InfraListener {
  id: string;
  name: string;
  host: string;
  port: string;
  protocol: string;
}

export interface Redirector {
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
  const [listeners, setListeners] = useState<InfraListener[]>([]);
  const [redirectors, setRedirectors] = useState<Redirector[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadListeners = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<{ Listeners?: InfraListener[]; listeners?: InfraListener[] }>("/listeners");
      setListeners((data.listeners || []) as InfraListener[]);
    } catch {
      setListeners([]);
      setError("Failed to load listeners");
      toast.error("Failed to load listeners");
    } finally {
      setLoading(false);
    }
  }, []);

  const loadRedirectors = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<{ redirectors?: Redirector[] }>("/redirectors");
      setRedirectors((data.redirectors || []) as Redirector[]);
    } catch {
      setRedirectors([]);
      setError("Failed to load redirectors");
      toast.error("Failed to load redirectors");
    } finally {
      setLoading(false);
    }
  }, []);

  return { listeners, redirectors, loading, error, loadListeners, loadRedirectors };
}

