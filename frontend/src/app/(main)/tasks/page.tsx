"use client";

import { Suspense, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

function TasksRedirectInner() {
  const router = useRouter();
  const sp = useSearchParams();
  useEffect(() => {
    const q = new URLSearchParams(sp.toString());
    q.set("tab", "tasks");
    router.replace(`/timeline?${q.toString()}`);
  }, [router, sp]);
  return null;
}

export default function TasksRedirect() {
  return (
    <Suspense fallback={null}>
      <TasksRedirectInner />
    </Suspense>
  );
}
