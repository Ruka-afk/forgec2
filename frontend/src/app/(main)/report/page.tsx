"use client";

import dynamic from "next/dynamic";

const ReportPageContent = dynamic(
  () => import("./ReportPageContent"),
  { ssr: false }
);

export default function ReportPagePage() {
  return <ReportPageContent />;
}