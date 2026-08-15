"use client";

import dynamic from "next/dynamic";

const PivotingPageContent = dynamic(
  () => import("./PivotingPageContent"),
  { ssr: false }
);

export default function PivotingPage() {
  return <PivotingPageContent />;
}
