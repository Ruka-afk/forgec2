"use client";

import dynamic from "next/dynamic";
import { Spinner } from "@/components/UI";
import type { Beacon } from "./_components/types";

export type { Beacon };

const AgentsPageContent = dynamic(
  () => import("./AgentsPageContent"),
  { ssr: false, loading: () => <div className="flex items-center justify-center h-64"><Spinner /></div> }
);

export default function AgentsPage() {
  return <AgentsPageContent />;
}
