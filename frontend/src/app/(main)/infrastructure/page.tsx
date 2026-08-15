"use client";

import dynamic from "next/dynamic";

const InfrastructurePageContent = dynamic(
  () => import("./InfrastructurePageContent"),
  { ssr: false }
);

export default function InfrastructurePagePage() {
  return <InfrastructurePageContent />;
}