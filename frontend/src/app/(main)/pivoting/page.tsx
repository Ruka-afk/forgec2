"use client";

import { lazy, Suspense } from "react";


const PivotingPageContent = lazy(
  () => import("./PivotingPageContent"));

export default function PivotingPage() {
  return <Suspense fallback={null}><PivotingPageContent /></Suspense>;
}
