"use client";

import dynamic from "next/dynamic";
import { Spinner } from "@/components/UI";

const PluginsPageContent = dynamic(
  () => import("./PluginsPageContent"),
  { ssr: false, loading: () => <div className="flex items-center justify-center h-64"><Spinner /></div> }
);

export default function PluginsPagePage() {
  return <PluginsPageContent />;
}