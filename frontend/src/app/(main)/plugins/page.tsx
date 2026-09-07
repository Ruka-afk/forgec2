"use client";

import { lazy, Suspense } from "react";


const PluginsPageContent = lazy(
  () => import("./PluginsPageContent"));

export default function PluginsPagePage() {
  return <Suspense fallback={null}><PluginsPageContent /></Suspense>;
}