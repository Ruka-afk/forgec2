"use client";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { User, X } from "lucide-react";
import { EVENT_COLORS, EVENT_TYPES } from "../page";

interface TimelineFiltersProps {
  selectedTypes: Set<string>;
  onToggleType: (type: string) => void;
  dateFrom: string;
  onDateFromChange: (val: string) => void;
  dateTo: string;
  onDateToChange: (val: string) => void;
  userFilter: string;
  onUserFilterChange: (val: string) => void;
  hasActiveFilters: boolean;
  onClearFilters: () => void;
}

function colorFor(type: string) {
  return EVENT_COLORS[type] || { dot: "bg-muted-foreground", bg: "bg-muted", text: "text-muted-foreground", icon: null };
}

export default function TimelineFilters({
  selectedTypes, onToggleType,
  dateFrom, onDateFromChange,
  dateTo, onDateToChange,
  userFilter, onUserFilterChange,
  hasActiveFilters, onClearFilters,
}: TimelineFiltersProps) {
  return (
    <div className="space-y-3">
      <Card className="p-4">
        <h3 className="text-xs font-semibold text-muted-foreground mb-3 uppercase tracking-wider">Event Types</h3>
        <div className="space-y-2">
          {EVENT_TYPES.map(type => {
            const color = colorFor(type);
            const isSelected = selectedTypes.has(type);
            return (
              <Label key={type} className="flex items-center gap-2 cursor-pointer group">
                <Checkbox
                  checked={isSelected}
                  onCheckedChange={() => onToggleType(type)}
                  aria-label={`Filter by ${type}`}
                />
                <span className={`w-2.5 h-2.5 rounded-full ${color.dot}`}></span>
                <span className="text-sm text-muted-foreground group-hover:text-foreground transition-colors">
                  {type.replace("_", " ").replace(/^\w/, c => c.toUpperCase())}
                </span>
              </Label>
            );
          })}
        </div>
      </Card>

      <Card className="p-4 space-y-3">
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Date Range</h3>
        <div>
          <Label className="text-xs mb-1">Start</Label>
          <Input type="date" value={dateFrom} onChange={e => onDateFromChange(e.target.value)} aria-label="Start date" />
        </div>
        <div>
          <Label className="text-xs mb-1">End</Label>
          <Input type="date" value={dateTo} onChange={e => onDateToChange(e.target.value)} aria-label="End date" />
        </div>
      </Card>

      <Card className="p-4 space-y-3">
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">User Filter</h3>
        <div className="relative">
          <User className="w-4 h-4" />
          <Input placeholder="Filter users..." value={userFilter} onChange={e => onUserFilterChange(e.target.value)} className="pl-9" aria-label="Filter by user" />
        </div>
      </Card>

      {hasActiveFilters && (
        <Button variant="outline" onClick={onClearFilters} className="w-full border-destructive/20 text-destructive hover:bg-destructive/10 gap-2">
          <X className="w-4 h-4" />
          <span>Clear All Filters</span>
        </Button>
      )}
    </div>
  );
}
