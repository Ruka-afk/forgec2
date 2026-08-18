/// <reference types="vite/client" />
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { Toaster } from "sonner";
import "./app/globals.css";
import ClientProvider from "@/components/ClientProvider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useTheme } from "@/lib/theme";
import { Root } from "./app/root";

function ThemedToaster() {
  const { resolved } = useTheme();
  return <Toaster theme={resolved} richColors />;
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <TooltipProvider>
        <ClientProvider>
          <Root />
          <ThemedToaster />
        </ClientProvider>
      </TooltipProvider>
    </BrowserRouter>
  </StrictMode>
);
