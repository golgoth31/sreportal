import { XIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export interface ActiveFilter {
  readonly label: string;
  readonly value: string;
  readonly onRemove: () => void;
}

interface FilterBarProps {
  searchValue: string;
  searchPlaceholder: string;
  searchAriaLabel: string;
  onSearchChange: (value: string) => void;
  hasFilters: boolean;
  onClearFilters: () => void;
  activeFilters: readonly ActiveFilter[];
  children?: ReactNode;
}

export function FilterBar({
  searchValue,
  searchPlaceholder,
  searchAriaLabel,
  onSearchChange,
  hasFilters,
  onClearFilters,
  activeFilters,
  children,
}: FilterBarProps) {
  return (
    <div className="space-y-3">
      {/* Controls row */}
      <div className="flex flex-wrap gap-3 items-end">
        <div className="flex-1 min-w-48">
          <Input
            placeholder={searchPlaceholder}
            value={searchValue}
            onChange={(e) => onSearchChange(e.target.value)}
            aria-label={searchAriaLabel}
          />
        </div>

        {children}

        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={onClearFilters}>
            <XIcon className="size-4" />
            Clear
          </Button>
        )}
      </div>

      {/* Active filter badges */}
      {hasFilters && activeFilters.length > 0 && (
        <div className="flex flex-wrap gap-1.5 items-center">
          <span className="text-muted-foreground text-xs">Filters:</span>
          {activeFilters.map((filter) => (
            <Badge
              key={filter.label}
              variant="secondary"
              className="text-xs gap-1"
            >
              {filter.label}: {filter.value}
              <button
                onClick={filter.onRemove}
                aria-label={`Remove ${filter.label} filter`}
              >
                <XIcon className="size-3" />
              </button>
            </Badge>
          ))}
        </div>
      )}
    </div>
  );
}
