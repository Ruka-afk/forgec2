"use client";

import { useEffect } from "react";
import { useParams } from "next/navigation";
import { subscribeAgent, unsubscribeAgent } from "@/lib/wsContext";

export default function AgentDetailLayout({ children }: { children: React.ReactNode }) {
  const params = useParams<{ id: string | string[] }>();
  const id = Array.isArray(params?.id) ? params.id[0] : params?.id;

  useEffect(() => {
    if (!id) return;
    subscribeAgent(id);
    return () => unsubscribeAgent(id);
  }, [id]);

  return <>{children}</>;
}
