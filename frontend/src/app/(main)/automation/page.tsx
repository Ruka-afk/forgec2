"use client";

import dynamic from "next/dynamic";

const AutomationPageContent = dynamic(
  () => import("./AutomationPageContent"),
  { ssr: false }
);

export default function AutomationPagePage() {
  return <AutomationPageContent />;
}