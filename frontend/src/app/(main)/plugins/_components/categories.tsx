"use client";

import { Anchor, Bug, Key, Layers, Search, Terminal } from "lucide-react";

export const PLUGIN_CATEGORIES = [
  { key: "", labelKey: "plugins.category_all", icon: <Layers className="w-4 h-4" />, color: "bg-secondary text-muted-foreground" },
  { key: "post-exploitation", labelKey: "plugins.category_post_exploitation", icon: <Terminal className="w-4 h-4" />, color: "bg-destructive/10 text-destructive" },
  { key: "reconnaissance", labelKey: "plugins.category_reconnaissance", icon: <Search className="w-4 h-4" />, color: "bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-400" },
  { key: "exploitation", labelKey: "plugins.category_exploitation", icon: <Bug className="w-4 h-4" />, color: "bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400" },
  { key: "credential", labelKey: "plugins.category_credential", icon: <Key className="w-4 h-4" />, color: "bg-primary/10 text-primary" },
  { key: "persistence", labelKey: "plugins.category_persistence", icon: <Anchor className="w-4 h-4" />, color: "bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400" },
] as const;
