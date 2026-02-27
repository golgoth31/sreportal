import { MonitorIcon, MoonIcon, SunIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useTheme, type ThemeMode } from "@/hooks/useTheme";

const ICONS: Record<ThemeMode, React.ReactNode> = {
  light: <SunIcon className="size-4" />,
  dark: <MoonIcon className="size-4" />,
  system: <MonitorIcon className="size-4" />,
};

const LABELS: Record<ThemeMode, string> = {
  light: "Light theme (click for dark)",
  dark: "Dark theme (click for system)",
  system: "System theme (click for light)",
};

export function ThemeToggle() {
  const { mode, toggle } = useTheme();

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          onClick={toggle}
          aria-label={LABELS[mode]}
        >
          {ICONS[mode]}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{LABELS[mode]}</TooltipContent>
    </Tooltip>
  );
}
