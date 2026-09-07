"use client";

import { Suspense, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

function FilesRedirectInner() {
  const navigate = useNavigate();
  const [sp] = useSearchParams();
  useEffect(() => {
    const id = sp.get("agent_id") || sp.get("id");
    navigate(id ? `/agents/${encodeURIComponent(id)}/files` : "/agents", { replace: true });
  }, [navigate, sp]);
  return null;
}

/** Global /files was a second browser that treated ls ACKs as listings. */
export default function FilesRedirect() {
  return (
    <Suspense fallback={null}>
      <FilesRedirectInner />
    </Suspense>
  );
}
