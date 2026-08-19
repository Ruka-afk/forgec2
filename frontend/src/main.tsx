/// <reference types="vite/client" />
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router-dom";
import { Toaster } from "sonner";
import "./app/globals.css";
import ClientProvider from "@/components/ClientProvider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useTheme } from "@/lib/theme";
import { router } from "./app/router";

function ThemedToaster() {
  const { resolved } = useTheme();
  return <Toaster theme={resolved} richColors />;
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <TooltipProvider>
      <ClientProvider>
        <RouterProvider router={router} />
        <ThemedToaster />
      </ClientProvider>
    </TooltipProvider>
  </StrictMode>
);
