"use client";

import dynamic from "next/dynamic";

const AIPageContent = dynamic(
  () => import("./AIPageContent"),
  { ssr: false }
);

export default function AIPagePage() {
  return <AIPageContent />;
}