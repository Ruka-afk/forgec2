"use client";

import { lazy, Suspense } from "react";


const ListenersPageContent = lazy(
  () => import("./ListenersPageContent"));

export default function ListenersPage() {
  return <Suspense fallback={null}><ListenersPageContent /></Suspense>;
}
