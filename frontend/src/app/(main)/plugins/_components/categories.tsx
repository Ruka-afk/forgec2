"use client";

import { Anchor, Bug, Key, Layers, Search, Terminal } from "lucide-react";

export const PLUGIN_CATEGORIES = [
  { key: "", labelKey: "plugins.category_all", icon: <Layers className="w-4 h-4" />, color: "bg-secondary text-muted-foreground" },
  { key: "post-exploitation", labelKey: "plugins.category_post_exploitation", icon: <Terminal className="w-4 h-4" />, color: "bg-destructive/10 text-destructive" },
  { key: "reconnaissance", labelKey: "plugins.category_reconnaissance", icon: <Search className="w-4 h-4" />, color: "bg-chart-2/15 text-chart-2" },
  { key: "exploitation", labelKey: "plugins.category_exploitation", icon: <Bug className="w-4 h-4" />, color: "bg-warning/15 text-warning" },
  { key: "credential", labelKey: "plugins.category_credential", icon: <Key className="w-4 h-4" />, color: "bg-primary/10 text-primary" },
  { key: "persistence", labelKey: "plugins.category_persistence", icon: <Anchor className="w-4 h-4" />, color: "bg-success/15 text-success" },
] as const;
