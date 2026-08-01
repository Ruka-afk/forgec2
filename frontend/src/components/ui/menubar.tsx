"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

interface MenubarContextValue {
  activeMenu: string | null;
  setActiveMenu: (id: string | null) => void;
  triggerRefs: React.MutableRefObject<Map<string, HTMLButtonElement>>;
  focusedIndex: number;
  setFocusedIndex: (i: number) => void;
  triggerCount: number;
  registerTrigger: () => number;
}

const MenubarContext = React.createContext<MenubarContextValue>({
  activeMenu: null,
  setActiveMenu: () => {},
  triggerRefs: { current: new Map() },
  focusedIndex: -1,
  setFocusedIndex: () => {},
  triggerCount: 0,
  registerTrigger: () => 0,
});

function useMenubarContext() {
  return React.useContext(MenubarContext);
}

type MenubarProps = React.ComponentProps<"div">;

function Menubar({ className, children, ...props }: MenubarProps) {
  const [activeMenu, setActiveMenu] = React.useState<string | null>(null);
  const [focusedIndex, setFocusedIndex] = React.useState(-1);
  const triggerRefs = React.useRef(new Map<string, HTMLButtonElement>());
  const triggerCountRef = React.useRef(0);

  const registerTrigger = React.useCallback(() => {
    const idx = triggerCountRef.current;
    triggerCountRef.current += 1;
    return idx;
  }, []);

  return (
    <MenubarContext.Provider value={{ activeMenu, setActiveMenu, triggerRefs, focusedIndex, setFocusedIndex, triggerCount: triggerCountRef.current, registerTrigger }}>
      <div
        role="menubar"
        aria-orientation="horizontal"
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
  const { activeMenu, setActiveMenu, focusedIndex, setFocusedIndex, triggerRefs, registerTrigger } = useMenubarContext();
  const isActive = activeMenu === id;
  const idxRef = React.useRef(registerTrigger());
  const btnRef = React.useRef<HTMLButtonElement>(null);

  React.useEffect(() => {
    if (btnRef.current) triggerRefs.current.set(id, btnRef.current);
    return () => { triggerRefs.current.delete(id); };
  }, [id, triggerRefs]);

  React.useEffect(() => {
    if (isActive && btnRef.current) btnRef.current.focus();
  }, [isActive]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    const triggers = Array.from(triggerRefs.current.entries());
    const currentIdx = triggers.findIndex(([key]) => key === id);
    if (e.key === "ArrowRight") {
      e.preventDefault();
      const next = (currentIdx + 1) % triggers.length;
      const [, nextBtn] = triggers[next];
      nextBtn?.focus();
      setFocusedIndex(next);
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      const prev = (currentIdx - 1 + triggers.length) % triggers.length;
      const [, prevBtn] = triggers[prev];
      prevBtn?.focus();
      setFocusedIndex(prev);
    } else if (e.key === "ArrowDown" || e.key === " " || e.key === "Enter") {
      e.preventDefault();
      setActiveMenu(isActive ? null : id);
    } else if (e.key === "Escape") {
      setActiveMenu(null);
    }
  };

  return (
    <button
      ref={btnRef}
      role="menuitem"
      aria-expanded={isActive}
      aria-haspopup="menu"
      tabIndex={idxRef.current === focusedIndex || (focusedIndex === -1 && idxRef.current === 0) ? 0 : -1}
      className={cn(
        "flex cursor-pointer items-center rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
        "hover:bg-accent hover:text-accent-foreground",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        isActive && "bg-accent text-accent-foreground",
        className
      )}
      onKeyDown={handleKeyDown}
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

const MenubarContentContext = React.createContext<{ registerItem: (el: HTMLDivElement) => void } | null>(null);

function MenubarContent({ className, align = "start", children, ...props }: MenubarContentProps) {
  const id = useMenubarMenu();
  const { activeMenu, setActiveMenu } = useMenubarContext();
  const ref = React.useRef<HTMLDivElement>(null);
  const itemRefs = React.useRef<HTMLDivElement[]>([]);

  const registerItem = React.useCallback((el: HTMLDivElement) => {
    if (!itemRefs.current.includes(el)) itemRefs.current.push(el);
  }, []);

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

  React.useEffect(() => {
    itemRefs.current = [];
    if (activeMenu === id) {
      requestAnimationFrame(() => itemRefs.current[0]?.focus());
    }
  }, [activeMenu, id]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    const items = itemRefs.current;
    const currentIdx = items.findIndex((el) => el === document.activeElement);

    if (e.key === "ArrowDown") {
      e.preventDefault();
      const next = (currentIdx + 1) % items.length;
      items[next]?.focus();
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prev = (currentIdx - 1 + items.length) % items.length;
      items[prev]?.focus();
    } else if (e.key === "Escape") {
      e.preventDefault();
      setActiveMenu(null);
    }
  };

  if (activeMenu !== id) return null;

  return (
    <MenubarContentContext.Provider value={{ registerItem }}>
      <div
        ref={ref}
        role="menu"
        onKeyDown={handleKeyDown}
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
    </MenubarContentContext.Provider>
  );
}

interface MenubarItemProps extends React.ComponentProps<"div"> {
  destructive?: boolean;
}

function MenubarItem({ className, destructive, onClick, ...props }: MenubarItemProps) {
  const { setActiveMenu } = useMenubarContext();
  const itemRef = React.useRef<HTMLDivElement>(null);
  const contentCtx = React.useContext(MenubarContentContext);

  React.useEffect(() => {
    if (itemRef.current && contentCtx) {
      contentCtx.registerItem(itemRef.current);
    }
  });

  return (
    <div
      ref={itemRef}
      role="menuitem"
      tabIndex={-1}
      className={cn(
        "flex cursor-pointer items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors",
        "hover:bg-accent hover:text-accent-foreground",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
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
