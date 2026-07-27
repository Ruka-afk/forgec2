"use client";

import dynamic from "next/dynamic";
import { Spinner } from "@/components/UI";

const ProfilesPageContent = dynamic(
  () => import("./ProfilesPageContent"),
  { ssr: false, loading: () => <div className="flex items-center justify-center h-64"><Spinner /></div> }
);

export default function ProfilesPagePage() {
  return <ProfilesPageContent />;
}