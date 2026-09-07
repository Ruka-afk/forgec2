"use client";

import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import type { GenerateTab } from "./generate-tabs";

export default function RedirectToGenerateTab({ tab }: { tab: GenerateTab }) {
  const navigate = useNavigate();
  useEffect(() => {
    navigate(`/generate?tab=${tab}`, { replace: true });
  }, [navigate, tab]);
  return null;
}
