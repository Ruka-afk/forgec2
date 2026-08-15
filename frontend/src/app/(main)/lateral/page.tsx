"use client";

import dynamic from "next/dynamic";

const LateralPageContent = dynamic(
  () => import("./LateralPageContent"),
  { ssr: false }
);

export default function LateralPage() {
  return <LateralPageContent />;
}
