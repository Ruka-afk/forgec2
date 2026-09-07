"use client";

import { Suspense, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

function NotificationsRedirectInner() {
  const navigate = useNavigate();
  const [sp] = useSearchParams();
  useEffect(() => {
    const q = new URLSearchParams(sp.toString());
    q.set("tab", "alerts");
    navigate(`/timeline?${q.toString()}`, { replace: true });
  }, [navigate, sp]);
  return null;
}

export default function NotificationsRedirect() {
  return (
    <Suspense fallback={null}>
      <NotificationsRedirectInner />
    </Suspense>
  );
}
