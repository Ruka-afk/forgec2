"use client";

import { lazy, Suspense } from "react";


const InfrastructurePageContent = lazy(
  () => import("./InfrastructurePageContent"));

export default function InfrastructurePagePage() {
  return <Suspense fallback={null}><InfrastructurePageContent /></Suspense>;
}