"use client";

import { useState, useRef, useEffect } from "react";

interface DropdownMenuProps {
  trigger: React.ReactNode;
  children: React.ReactNode;
  align?: "left" | "right";
  className?: string;
}

export default function DropdownMenu({ trigger, children, align = "right", className = "" }: DropdownMenuProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  return (
    <div className={`relative ${className}`} ref={ref}>
      <div onClick={() => setOpen(!open)}>{trigger}</div>
      {open && (
        <div
          className={`absolute top-full mt-1 z-50 bg-[var(--card-bg)] border border-[var(--border)] rounded-xl shadow-lg py-1 min-w-[10rem] animate-fade-in ${
            align === "right" ? "right-0" : "left-0"
          }`}
          onClick={() => setOpen(false)}
        >
          {children}
        </div>
      )}
    </div>
  );
}

export function DropdownItem({ onClick, active, icon, children, danger }: {
  onClick?: () => void;
  active?: boolean;
  icon?: string;
  children: React.ReactNode;
  danger?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      className={`w-full flex items-center gap-2 px-3 py-2 text-sm transition-colors ${
        danger
          ? "text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20"
          : active
            ? "text-indigo-600 dark:text-indigo-400 bg-indigo-50/50 dark:bg-indigo-900/10"
            : "text-[var(--text-secondary)] hover:bg-[var(--card-bg-secondary)]"
      }`}
    >
      {icon && <i className={`${icon} text-xs w-4`}></i>}
      {children}
    </button>
  );
}

export function DropdownDivider() {
  return <div className="border-t border-[var(--border)] my-1" />;
}

export function DropdownHeader({ children }: { children: React.ReactNode }) {
  return (
    <div className="px-4 py-3 border-b border-[var(--border)]">
      <div className="text-sm font-medium text-[var(--text-primary)]">{children}</div>
    </div>
  );
}
