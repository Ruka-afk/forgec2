"use client";

import { lazy, Suspense } from "react";


const ReportPageContent = lazy(
  () => import("./ReportPageContent"));

export default function ReportPagePage() {
  return <Suspense fallback={null}><ReportPageContent /></Suspense>;
}