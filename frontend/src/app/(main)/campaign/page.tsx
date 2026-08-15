"use client";

import dynamic from "next/dynamic";

const CampaignPageContent = dynamic(
  () => import("./CampaignPageContent"),
  { ssr: false }
);

export default function CampaignsPage() {
  return <CampaignPageContent />;
}
