import { format, parse, subDays } from "date-fns";
import { CalendarIcon } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ErrorAlert } from "@/components/ErrorAlert";
import { FilterBar, type ActiveFilter } from "@/components/FilterBar";
import { useReleaseDays } from "@/features/release/hooks/useReleaseDays";
import { useReleases } from "@/features/release/hooks/useReleases";
import { ReleaseList } from "@/features/release/ui/ReleaseList";

const DAY_FORMAT = "yyyy-MM-dd";

function parseDayToDate(day: string): Date | undefined {
  if (!day) return undefined;
  return parse(day, DAY_FORMAT, new Date());
}

export function ReleasesPage() {
  const {
    day,
    entries,
    totalCount,
    isLoading,
    error,
    search,
    setSearch,
    goToDay,
    clearFilters,
    hasFilters,
  } = useReleases();

  const { isDayDisabled, ttlDays } = useReleaseDays();

  const [calendarOpen, setCalendarOpen] = useState(false);

  const selectedDate = useMemo(() => parseDayToDate(day), [day]);

  const today = useMemo(() => new Date(), []);
  const fromDate = useMemo(
    () => (ttlDays > 0 ? subDays(today, ttlDays) : undefined),
    [today, ttlDays],
  );

  const handleDateSelect = useCallback(
    (date: Date | undefined) => {
      if (date) {
        goToDay(format(date, DAY_FORMAT));
      }
      setCalendarOpen(false);
    },
    [goToDay],
  );

  const activeFilters = useMemo((): ActiveFilter[] => {
    const filters: ActiveFilter[] = [];
    if (search) {
      filters.push({
        label: "search",
        value: search,
        onRemove: () => setSearch(""),
      });
    }
    return filters;
  }, [search, setSearch]);

  return (
    <div className="max-w-screen-xl mx-auto px-4 py-6 space-y-6">
      {/* Header with day navigation */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h1 className="text-xl font-semibold tracking-tight">Releases</h1>

        {!isLoading && (
          <div className="flex items-center gap-2 ml-auto">
            {/* Date picker */}
            <Popover open={calendarOpen} onOpenChange={setCalendarOpen}>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  className="min-w-36 justify-start gap-2 font-mono text-sm"
                  aria-label="Pick a date"
                >
                  <CalendarIcon className="size-4" />
                  {day || "Select date"}
                </Button>
              </PopoverTrigger>
              <PopoverContent className="w-auto p-0" align="center">
                <Calendar
                  mode="single"
                  selected={selectedDate}
                  onSelect={handleDateSelect}
                  defaultMonth={selectedDate}
                  disabled={isDayDisabled}
                  fromDate={fromDate}
                  toDate={today}
                />
              </PopoverContent>
            </Popover>

            <span className="text-muted-foreground text-sm ml-2">
              {hasFilters
                ? `${entries.length} / ${totalCount} release(s)`
                : `${totalCount} release(s)`}
            </span>
          </div>
        )}
      </div>

      {/* Search */}
      <FilterBar
        searchValue={search}
        searchPlaceholder="Search by type, version, origin, author..."
        searchAriaLabel="Search releases"
        onSearchChange={setSearch}
        hasFilters={hasFilters}
        onClearFilters={clearFilters}
        activeFilters={activeFilters}
      />

      {/* Error state */}
      {error && <ErrorAlert title="Failed to load releases" error={error} />}

      {/* Release list */}
      {!error && (
        <ReleaseList
          entries={entries}
          isLoading={isLoading}
          hasFilters={hasFilters}
          onClearFilters={clearFilters}
        />
      )}
    </div>
  );
}
