"use client";

import dynamic from "next/dynamic";

const PluginsPageContent = dynamic(
  () => import("./PluginsPageContent"),
  { ssr: false }
);

export default function PluginsPagePage() {
  return <PluginsPageContent />;
}