"use client";

import dynamic from "next/dynamic";

const PhishingPageContent = dynamic(
  () => import("./PhishingPageContent"),
  { ssr: false }
);

export default function PhishingPage() {
  return <PhishingPageContent />;
}
