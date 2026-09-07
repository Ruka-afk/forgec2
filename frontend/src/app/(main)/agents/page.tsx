"use client";

import { lazy, Suspense } from "react";

import type { Beacon } from "./_components/types";

export type { Beacon };

const AgentsPageContent = lazy(
  () => import("./AgentsPageContent"));

export default function AgentsPage() {
  return <Suspense fallback={null}><AgentsPageContent /></Suspense>;
}
