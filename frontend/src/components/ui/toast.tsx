"use client";

import { Toaster as SonnerToaster, toast } from "sonner";

type ToastProps = React.ComponentProps<typeof SonnerToaster>;

function ToastProvider({ ...props }: ToastProps) {
  return <SonnerToaster {...props} />;
}

export { ToastProvider, toast };
