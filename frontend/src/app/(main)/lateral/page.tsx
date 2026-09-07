"use client";

import { lazy, Suspense } from "react";


const LateralPageContent = lazy(
  () => import("./LateralPageContent"));

export default function LateralPage() {
  return <Suspense fallback={null}><LateralPageContent /></Suspense>;
}
