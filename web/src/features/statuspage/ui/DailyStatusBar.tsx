import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

import type { DailyStatus } from "../domain/types";
import { getStatusDotColor, getStatusLabel } from "../domain/utils";

interface DailyStatusBarProps {
  dailyStatus: DailyStatus[];
}

export function DailyStatusBar({ dailyStatus }: DailyStatusBarProps) {
  if (dailyStatus.length === 0) return null;

  return (
    <div className="flex gap-px mt-2">
      {dailyStatus.map((day) => (
        <Tooltip key={day.date}>
          <TooltipTrigger asChild>
            <div
              className={`h-5 flex-1 rounded-sm ${getStatusDotColor(day.worstStatus)} opacity-80 hover:opacity-100 transition-opacity cursor-default`}
            />
          </TooltipTrigger>
          <TooltipContent side="top" className="text-xs">
            <p className="font-medium">{day.date}</p>
            <p className="text-muted-foreground">
              {getStatusLabel(day.worstStatus)}
            </p>
          </TooltipContent>
        </Tooltip>
      ))}
    </div>
  );
}
