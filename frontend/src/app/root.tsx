import { Suspense } from "react";
import { PageSpinner } from "@/components/ui/spinner";
import { AppRoutes } from "./router";

export function Root() {
  return (
    <Suspense fallback={<PageSpinner />}>
      <AppRoutes />
    </Suspense>
  );
}
