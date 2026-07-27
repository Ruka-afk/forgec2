"use client";

import dynamic from "next/dynamic";
import { Spinner } from "@/components/UI";

const LateralPageContent = dynamic(
  () => import("./LateralPageContent"),
  { ssr: false, loading: () => <div className="flex items-center justify-center h-64"><Spinner /></div> }
);

export default function LateralPage() {
  return <LateralPageContent />;
}
