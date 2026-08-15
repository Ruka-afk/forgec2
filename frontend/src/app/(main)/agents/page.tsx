"use client";

import dynamic from "next/dynamic";
import type { Beacon } from "./_components/types";

export type { Beacon };

const AgentsPageContent = dynamic(
  () => import("./AgentsPageContent"),
  { ssr: false }
);

export default function AgentsPage() {
  return <AgentsPageContent />;
}
