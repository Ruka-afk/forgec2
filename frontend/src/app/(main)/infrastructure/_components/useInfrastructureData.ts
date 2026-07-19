import { useState, useCallback } from "react";
import { api } from "@/lib/api";

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

  const loadListeners = useCallback(async () => {
    try {
      const data = await api.get<{ Listeners?: InfraListener[]; listeners?: InfraListener[] }>("/listeners");
      setListeners((data.listeners || []) as InfraListener[]);
    } catch {
      setListeners([]);
    }
  }, []);

  const loadRedirectors = useCallback(async () => {
    try {
      const data = await api.get<{ redirectors?: Redirector[] }>("/redirectors");
      setRedirectors((data.redirectors || []) as Redirector[]);
    } catch {
      setRedirectors([]);
    }
  }, []);

  return { listeners, redirectors, loadListeners, loadRedirectors };
}

