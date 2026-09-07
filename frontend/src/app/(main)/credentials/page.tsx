"use client";

import { lazy, Suspense } from "react";


const CredentialsPageContent = lazy(
  () => import("./CredentialsPageContent"));

export default function CredentialsPagePage() {
  return <Suspense fallback={null}><CredentialsPageContent /></Suspense>;
}