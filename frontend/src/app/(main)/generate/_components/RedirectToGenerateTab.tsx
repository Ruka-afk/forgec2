"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import type { GenerateTab } from "./generate-tabs";

export default function RedirectToGenerateTab({ tab }: { tab: GenerateTab }) {
  const router = useRouter();
  useEffect(() => {
    router.replace(`/generate?tab=${tab}`);
  }, [router, tab]);
  return null;
}
