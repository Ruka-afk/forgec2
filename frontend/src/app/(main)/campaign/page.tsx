"use client";

import { lazy, Suspense } from "react";


const CampaignPageContent = lazy(
  () => import("./CampaignPageContent"));

export default function CampaignsPage() {
  return <Suspense fallback={null}><CampaignPageContent /></Suspense>;
}
