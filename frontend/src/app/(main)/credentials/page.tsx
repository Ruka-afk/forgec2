"use client";

import dynamic from "next/dynamic";

const CredentialsPageContent = dynamic(
  () => import("./CredentialsPageContent"),
  { ssr: false }
);

export default function CredentialsPagePage() {
  return <CredentialsPageContent />;
}