"use client";

import { lazy, Suspense } from "react";


const AutomationPageContent = lazy(
  () => import("./AutomationPageContent"));

export default function AutomationPagePage() {
  return <Suspense fallback={null}><AutomationPageContent /></Suspense>;
}