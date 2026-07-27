import type { ReactNode } from "react";

interface PageLayoutProps {
  children: ReactNode;
  className?: string;
}

export default function PageLayout({ children, className = "" }: PageLayoutProps) {
  return (
    <div className={`max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up ${className}`}>
      {children}
    </div>
  );
}
