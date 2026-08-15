"use client";

import { Suspense, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

function FilesRedirectInner() {
  const router = useRouter();
  const sp = useSearchParams();
  useEffect(() => {
    const id = sp.get("agent_id") || sp.get("id");
    router.replace(id ? `/agents/${encodeURIComponent(id)}/files` : "/agents");
  }, [router, sp]);
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
