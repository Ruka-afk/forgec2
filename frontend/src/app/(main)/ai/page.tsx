"use client";

import { lazy, Suspense } from "react";


const AIPageContent = lazy(
  () => import("./AIPageContent"));

export default function AIPagePage() {
  return <Suspense fallback={null}><AIPageContent /></Suspense>;
}