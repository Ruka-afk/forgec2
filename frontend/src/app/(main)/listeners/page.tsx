"use client";

import dynamic from "next/dynamic";

const ListenersPageContent = dynamic(
  () => import("./ListenersPageContent"),
  { ssr: false }
);

export default function ListenersPage() {
  return <ListenersPageContent />;
}
