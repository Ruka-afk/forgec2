"use client";

import { Suspense, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

function NotificationsRedirectInner() {
  const router = useRouter();
  const sp = useSearchParams();
  useEffect(() => {
    const q = new URLSearchParams(sp.toString());
    q.set("tab", "alerts");
    router.replace(`/timeline?${q.toString()}`);
  }, [router, sp]);
  return null;
}

export default function NotificationsRedirect() {
  return (
    <Suspense fallback={null}>
      <NotificationsRedirectInner />
    </Suspense>
  );
}
