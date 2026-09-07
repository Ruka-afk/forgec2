"use client";

import { lazy, Suspense } from "react";


const PhishingPageContent = lazy(
  () => import("./PhishingPageContent"));

export default function PhishingPage() {
  return <Suspense fallback={null}><PhishingPageContent /></Suspense>;
}
