"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

interface MenubarContextValue {
  activeMenu: string | null;
  setActiveMenu: (id: string | null) => void;
}

const MenubarContext = React.createContext<MenubarContextValue>({
  activeMenu: null,
  setActiveMenu: () => {},
});

function useMenubarContext() {
  return React.useContext(MenubarContext);
}

type MenubarProps = React.ComponentProps<"div">;

function Menubar({ className, children, ...props }: MenubarProps) {
  const [activeMenu, setActiveMenu] = React.useState<string | null>(null);
  return (
    <MenubarContext.Provider value={{ activeMenu, setActiveMenu }}>
      <div
        role="menubar"
        className={cn(
          "flex items-center gap-1 rounded-lg border border-border bg-card p-1 shadow-sm",
          className
        )}
        {...props}
      >
        {children}
      </div>
    </MenubarContext.Provider>
  );
}

interface MenubarMenuProps {
  children: React.ReactNode;
  value?: string;
}

function MenubarMenu({ children, value }: MenubarMenuProps) {
  const menuValue = React.useId();
  const id = value || menuValue;
  return <MenubarMenuContext.Provider value={id}>{children}</MenubarMenuContext.Provider>;
}

const MenubarMenuContext = React.createContext<string>("");

function useMenubarMenu() {
  return React.useContext(MenubarMenuContext);
}

type MenubarTriggerProps = React.ComponentProps<"button">;

function MenubarTrigger({ className, onClick, ...props }: MenubarTriggerProps) {
  const id = useMenubarMenu();
  const { activeMenu, setActiveMenu } = useMenubarContext();
  const isActive = activeMenu === id;

  return (
    <button
      role="menuitem"
      aria-expanded={isActive}
      className={cn(
        "flex cursor-pointer items-center rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
        "hover:bg-accent hover:text-accent-foreground",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        isActive && "bg-accent text-accent-foreground",
        className
      )}
      onClick={(e) => {
        setActiveMenu(isActive ? null : id);
        onClick?.(e);
      }}
      {...props}
    />
  );
}

interface MenubarContentProps extends React.ComponentProps<"div"> {
  align?: "start" | "center" | "end";
}

function MenubarContent({ className, align = "start", children, ...props }: MenubarContentProps) {
  const id = useMenubarMenu();
  const { activeMenu, setActiveMenu } = useMenubarContext();
  const ref = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    if (activeMenu !== id) return;
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setActiveMenu(null);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [activeMenu, id, setActiveMenu]);

  if (activeMenu !== id) return null;

  return (
    <div
      ref={ref}
      role="menu"
      className={cn(
        "absolute z-50 mt-1 min-w-[12rem] overflow-hidden rounded-lg border border-border bg-popover p-1 shadow-lg animate-scale-in",
        align === "end" && "right-0",
        align === "center" && "left-1/2 -translate-x-1/2",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

interface MenubarItemProps extends React.ComponentProps<"div"> {
  destructive?: boolean;
}

function MenubarItem({ className, destructive, onClick, ...props }: MenubarItemProps) {
  const { setActiveMenu } = useMenubarContext();
  return (
    <div
      role="menuitem"
      className={cn(
        "flex cursor-pointer items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors",
        "hover:bg-accent hover:text-accent-foreground",
        destructive && "text-destructive hover:bg-destructive/10 hover:text-destructive",
        className
      )}
      onClick={(e) => {
        onClick?.(e);
        setActiveMenu(null);
      }}
      {...props}
    />
  );
}

function MenubarSeparator({ className }: { className?: string }) {
  return <div className={cn("my-1 h-px bg-border", className)} />;
}

function MenubarLabel({ className, children, ...props }: React.ComponentProps<"div">) {
  return (
    <div className={cn("px-3 py-1.5 text-xs font-semibold text-muted-foreground", className)} {...props}>
      {children}
    </div>
  );
}

export { Menubar, MenubarMenu, MenubarTrigger, MenubarContent, MenubarItem, MenubarSeparator, MenubarLabel };
